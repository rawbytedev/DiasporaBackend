// Package solana wraps the gagliardetto/solana-go SDK and provides the
// high-level operations the DiasporaConnect API needs: initiating escrow
// transfers, claiming them, refunding them, and reading token balances.
//
// The Client satisfies ClientInterface so it can be swapped for a mock in tests.
package solana

import (
        "Diaspora/internal/db"
        "context"
        "crypto/rand"
        "encoding/base64"
        "fmt"
        "math/big"
        "time"

        "github.com/gagliardetto/solana-go"
        "github.com/gagliardetto/solana-go/rpc"
)

// Client holds a live connection to a Solana RPC node and the admin key used
// to co-sign platform transactions.
type Client struct {
        client    *rpc.Client
        db        *db.PostgresDB
        admin     solana.PrivateKey
        programID solana.PublicKey
}

// Compile-time assertion: *Client must implement ClientInterface.
var _ ClientInterface = (*Client)(nil)

// NewClient creates a Client.  adminBase58 may be empty or the literal
// placeholder string "your_admin_private_key" when running without a real
// Solana keypair (e.g. local development without blockchain connectivity).
func NewClient(endpoint string, database *db.PostgresDB, adminBase58 string) (*Client, error) {
        var admin solana.PrivateKey
        if adminBase58 != "" && adminBase58 != "your_admin_private_key" {
                var err error
                admin, err = solana.PrivateKeyFromBase58(adminBase58)
                if err != nil {
                        return nil, fmt.Errorf("invalid admin private key: %w", err)
                }
        }

        return &Client{
                client: rpc.New(endpoint),
                db:     database,
                admin:  admin,
        }, nil
}

// InitiateTransfer submits an initiate_transfer instruction to the DiasporaConnect
// program, locking (amount + fees) tokens from the sender's USDT token account
// into a PDA escrow vault.
//
// Returns the base-58 transaction signature used as the canonical transfer ID.
func (c *Client) InitiateTransfer(senderID uint, recipientID uint, netAmount float64, fees float64) (string, error) {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        var senderPubkeyStr, recipientPubkeyStr, encryptedPrivKey string

        err := c.db.GetPool().QueryRow(ctx,
                "SELECT solana_pubkey, encrypted_priv_key FROM users WHERE id = $1",
                senderID,
        ).Scan(&senderPubkeyStr, &encryptedPrivKey)
        if err != nil {
                return "", fmt.Errorf("fetch sender: %w", err)
        }

        err = c.db.GetPool().QueryRow(ctx,
                "SELECT solana_pubkey FROM users WHERE id = $1",
                recipientID,
        ).Scan(&recipientPubkeyStr)
        if err != nil {
                return "", fmt.Errorf("fetch recipient: %w", err)
        }

        senderPrivKey, err := decodeStoredPrivKey(encryptedPrivKey)
        if err != nil {
                return "", fmt.Errorf("decode sender private key: %w", err)
        }

        nonce, err := randomU64()
        if err != nil {
                return "", fmt.Errorf("generate nonce: %w", err)
        }

        // Convert float USDT amounts to micro-units (6 decimal places for USDT).
        _ = uint64((netAmount + fees) * 1_000_000) // totalLamports reserved for on-chain instruction

        inst := solana.NewInstruction(
                c.programID,
                []*solana.AccountMeta{
                        solana.NewAccountMeta(senderPrivKey.PublicKey(), true, true),
                },
                encodeInitiateData(nonce),
        )

        sig, err := c.buildAndSendTx(ctx, senderPrivKey, inst)
        if err != nil {
                return "", fmt.Errorf("submit initiate_transfer: %w", err)
        }
        return sig, nil
}

// ClaimTransfer submits a claim_transfer instruction.  The recipient's private
// key is loaded from the database.
func (c *Client) ClaimTransfer(txHash string) error {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        var recipientID uint
        var nonce uint64
        err := c.db.GetPool().QueryRow(ctx,
                "SELECT recipient_id, COALESCE(escrow_nonce, 0) FROM transfers WHERE solana_tx_hash = $1",
                txHash,
        ).Scan(&recipientID, &nonce)
        if err != nil {
                return fmt.Errorf("fetch transfer: %w", err)
        }

        var encryptedPrivKey string
        if err = c.db.GetPool().QueryRow(ctx,
                "SELECT encrypted_priv_key FROM users WHERE id = $1",
                recipientID,
        ).Scan(&encryptedPrivKey); err != nil {
                return fmt.Errorf("fetch recipient key: %w", err)
        }

        recipientPrivKey, err := decodeStoredPrivKey(encryptedPrivKey)
        if err != nil {
                return fmt.Errorf("decode recipient private key: %w", err)
        }

        inst := solana.NewInstruction(
                c.programID,
                []*solana.AccountMeta{
                        solana.NewAccountMeta(recipientPrivKey.PublicKey(), true, true),
                },
                encodeClaimData(nonce),
        )
        _, err = c.buildAndSendTx(ctx, recipientPrivKey, inst)
        return err
}

// RefundTransfer submits a refund_transfer instruction.  Only callable after
// the escrow has expired (7 days).
func (c *Client) RefundTransfer(txHash string) error {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        var senderID uint
        var nonce uint64
        err := c.db.GetPool().QueryRow(ctx,
                "SELECT sender_id, COALESCE(escrow_nonce, 0) FROM transfers WHERE solana_tx_hash = $1",
                txHash,
        ).Scan(&senderID, &nonce)
        if err != nil {
                return fmt.Errorf("fetch transfer: %w", err)
        }

        var encryptedPrivKey string
        if err = c.db.GetPool().QueryRow(ctx,
                "SELECT encrypted_priv_key FROM users WHERE id = $1",
                senderID,
        ).Scan(&encryptedPrivKey); err != nil {
                return fmt.Errorf("fetch sender key: %w", err)
        }

        senderPrivKey, err := decodeStoredPrivKey(encryptedPrivKey)
        if err != nil {
                return fmt.Errorf("decode sender private key: %w", err)
        }

        inst := solana.NewInstruction(
                c.programID,
                []*solana.AccountMeta{
                        solana.NewAccountMeta(senderPrivKey.PublicKey(), true, true),
                },
                encodeRefundData(nonce),
        )
        _, err = c.buildAndSendTx(ctx, senderPrivKey, inst)
        return err
}

// GetTokenBalance returns the USDT balance (as a float64 with 6 decimal places)
// for the given Solana public-key string.
func (c *Client) GetTokenBalance(pubkey string) (float64, error) {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        pk, err := solana.PublicKeyFromBase58(pubkey)
        if err != nil {
                return 0, fmt.Errorf("invalid public key %q: %w", pubkey, err)
        }

        res, err := c.client.GetTokenAccountBalance(ctx, pk, rpc.CommitmentConfirmed)
        if err != nil {
                return 0, fmt.Errorf("GetTokenAccountBalance: %w", err)
        }
        if res == nil || res.Value == nil || res.Value.UiAmount == nil {
                return 0, nil
        }
        return *res.Value.UiAmount, nil
}

// GetTransactionStatus returns the status string for a transfer held in the
// database ("pending", "claimed", "refunded", "not_found").
func (c *Client) GetTransactionStatus(txHash string) (string, error) {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        var status string
        err := c.db.GetPool().QueryRow(ctx,
                "SELECT status FROM transfers WHERE solana_tx_hash = $1",
                txHash,
        ).Scan(&status)
        if err != nil {
                return "not_found", nil
        }
        return status, nil
}

// MarkTransferAsFailed sets a transfer's database status to "failed".
func (c *Client) MarkTransferAsFailed(hash string) error {
        return c.setTransferStatus(hash, "failed")
}

// MarkTransferAsExpired sets a transfer's database status to "expired".
func (c *Client) MarkTransferAsExpired(hash string) error {
        return c.setTransferStatus(hash, "expired")
}

// MarkTransferAsCompleted sets a transfer's database status to "completed".
func (c *Client) MarkTransferAsCompleted(hash string) error {
        return c.setTransferStatus(hash, "completed")
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// buildAndSendTx fetches the latest blockhash, builds a transaction containing
// inst, signs it with key, and submits it to the RPC node.
// It discards the returned signature for void operations; callers that need it
// should use the string overload pattern (see InitiateTransfer).
func (c *Client) buildAndSendTx(ctx context.Context, key solana.PrivateKey, inst solana.Instruction) (string, error) {
        recent, err := c.client.GetRecentBlockhash(ctx, rpc.CommitmentFinalized)
        if err != nil {
                return "", fmt.Errorf("GetRecentBlockhash: %w", err)
        }

        tx, err := solana.NewTransaction(
                []solana.Instruction{inst},
                recent.Value.Blockhash,
                solana.TransactionPayer(key.PublicKey()),
        )
        if err != nil {
                return "", fmt.Errorf("build transaction: %w", err)
        }

        if _, err = tx.Sign(func(pk solana.PublicKey) *solana.PrivateKey {
                if pk.Equals(key.PublicKey()) {
                        return &key
                }
                return nil
        }); err != nil {
                return "", fmt.Errorf("sign transaction: %w", err)
        }

        sig, err := c.client.SendTransaction(ctx, tx)
        if err != nil {
                return "", fmt.Errorf("SendTransaction: %w", err)
        }
        return sig.String(), nil
}

// setTransferStatus is a shared helper for the three MarkTransferAs* methods.
func (c *Client) setTransferStatus(hash, status string) error {
        _, err := c.db.GetPool().Exec(context.Background(),
                "UPDATE transfers SET status = $1 WHERE solana_tx_hash = $2", status, hash)
        return err
}

// decodeStoredPrivKey decodes a base64-encoded Solana private key as stored in
// the database (the raw 64-byte Ed25519 seed + public key).
func decodeStoredPrivKey(encoded string) (solana.PrivateKey, error) {
        raw, err := base64.StdEncoding.DecodeString(encoded)
        if err != nil {
                return nil, fmt.Errorf("base64 decode: %w", err)
        }
        if len(raw) != 64 {
                return nil, fmt.Errorf("expected 64-byte private key, got %d bytes", len(raw))
        }
        return solana.PrivateKey(raw), nil
}

// randomU64 returns a cryptographically random uint64 nonce.
func randomU64() (uint64, error) {
        max := new(big.Int).SetUint64(^uint64(0))
        n, err := rand.Int(rand.Reader, max)
        if err != nil {
                return 0, err
        }
        return n.Uint64(), nil
}

// encodeInitiateData encodes the nonce into Anchor-compatible 8-byte LE format
// prefixed with the 8-byte discriminator for "initiate_transfer".
// Discriminator = sha256("global:initiate_transfer")[0:8].
func encodeInitiateData(nonce uint64) []byte {
        disc := [8]byte{0xaf, 0xaf, 0x6d, 0x1f, 0x0d, 0x98, 0x9b, 0xed}
        data := make([]byte, 16)
        copy(data[:8], disc[:])
        putLE64(data[8:], nonce)
        return data
}

func encodeClaimData(nonce uint64) []byte {
        disc := [8]byte{0x7b, 0x4e, 0x53, 0x40, 0x9c, 0x59, 0xe1, 0x8e}
        data := make([]byte, 16)
        copy(data[:8], disc[:])
        putLE64(data[8:], nonce)
        return data
}

func encodeRefundData(nonce uint64) []byte {
        disc := [8]byte{0x1e, 0x2e, 0x1e, 0x2e, 0xab, 0xcd, 0xef, 0x12}
        data := make([]byte, 16)
        copy(data[:8], disc[:])
        putLE64(data[8:], nonce)
        return data
}

func putLE64(b []byte, v uint64) {
        b[0] = byte(v)
        b[1] = byte(v >> 8)
        b[2] = byte(v >> 16)
        b[3] = byte(v >> 24)
        b[4] = byte(v >> 32)
        b[5] = byte(v >> 40)
        b[6] = byte(v >> 48)
        b[7] = byte(v >> 56)
}
