// Package solana wraps the gagliardetto/solana-go SDK and provides the
// high-level operations the DiasporaConnect API needs.
//
// Each public method builds a fully-specified Anchor instruction (correct
// discriminator, all required accounts, borsh-encoded arguments) and submits
// it to the configured RPC node.  The Client satisfies ClientInterface so it
// can be swapped for a mock in tests.
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

// ---------------------------------------------------------------------------
// Well-known Solana program addresses
// ---------------------------------------------------------------------------

var (
	splTokenProgram = solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	ataProgram      = solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJe8bv")
)

// ---------------------------------------------------------------------------
// Anchor discriminators
//
// sha256("global:<instruction_name>")[0:8] — pre-computed and verified by
// internal/solana/solana_test.go (TestDiscriminators).
// ---------------------------------------------------------------------------

var (
	discInitiateTransfer = [8]byte{0x80, 0xe5, 0x4d, 0x05, 0x41, 0xea, 0xe4, 0x4b}
	discClaimTransfer    = [8]byte{0xca, 0xb2, 0x3a, 0xbe, 0xe6, 0xea, 0xe5, 0x11}
	discRefundTransfer   = [8]byte{0x62, 0xaf, 0x73, 0xa9, 0x14, 0x3f, 0x66, 0xe2}
)

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

// Client holds a live connection to a Solana RPC node together with the
// configuration needed to build DiasporaConnect program instructions.
type Client struct {
	rpc            *rpc.Client
	db             *db.PostgresDB
	admin          solana.PrivateKey
	programID      solana.PublicKey
	usdtMint       solana.PublicKey
	treasuryWallet solana.PublicKey // wallet that owns the fee-treasury token account
}

// Compile-time assertion: *Client must implement ClientInterface.
var _ ClientInterface = (*Client)(nil)

// NewClient creates a Client.
//
//   - adminBase58      – platform co-signer key; may be empty in development
//   - programIDBase58  – deployed DiasporaConnect program ID
//   - usdtMintBase58   – USDT mint address on the target cluster
//   - treasuryBase58   – treasury wallet public key (owner of the fee ATA)
func NewClient(
	endpoint string,
	database *db.PostgresDB,
	adminBase58, programIDBase58, usdtMintBase58, treasuryBase58 string,
) (*Client, error) {
	var admin solana.PrivateKey
	if adminBase58 != "" && adminBase58 != "your_admin_private_key" {
		var err error
		admin, err = solana.PrivateKeyFromBase58(adminBase58)
		if err != nil {
			return nil, fmt.Errorf("invalid admin private key: %w", err)
		}
	}

	programID := solana.MustPublicKeyFromBase58("5GHE14Zmpq5yNwpvHR2ZLaTcSckp6QogCRNm43M3Z9BT")
	if programIDBase58 != "" {
		pk, err := solana.PublicKeyFromBase58(programIDBase58)
		if err != nil {
			return nil, fmt.Errorf("invalid program ID: %w", err)
		}
		programID = pk
	}

	var usdtMint solana.PublicKey
	if usdtMintBase58 != "" {
		pk, err := solana.PublicKeyFromBase58(usdtMintBase58)
		if err != nil {
			return nil, fmt.Errorf("invalid USDT mint: %w", err)
		}
		usdtMint = pk
	}

	var treasuryWallet solana.PublicKey
	if treasuryBase58 != "" {
		pk, err := solana.PublicKeyFromBase58(treasuryBase58)
		if err != nil {
			return nil, fmt.Errorf("invalid treasury wallet: %w", err)
		}
		treasuryWallet = pk
	}

	return &Client{
		rpc:            rpc.New(endpoint),
		db:             database,
		admin:          admin,
		programID:      programID,
		usdtMint:       usdtMint,
		treasuryWallet: treasuryWallet,
	}, nil
}

// ---------------------------------------------------------------------------
// ClientInterface implementation
// ---------------------------------------------------------------------------

// InitiateTransfer submits an initiate_transfer instruction to the
// DiasporaConnect program, locking (netAmount + fees) USDT tokens from the
// sender's ATA into a PDA escrow vault.
//
// The nonce returned must be stored by the caller alongside the transaction
// hash so that ClaimTransfer and RefundTransfer can reconstruct the PDAs.
func (c *Client) InitiateTransfer(senderID, recipientID uint, netAmount, fees float64) (string, uint64, error) {
	if err := c.requireMintAndTreasury(); err != nil {
		return "", 0, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Load sender keypair and recipient pubkey from the database.
	var senderPubkeyStr, encSenderKey string
	if err := c.db.GetPool().QueryRow(ctx,
		"SELECT solana_pubkey, encrypted_priv_key FROM users WHERE id = $1", senderID,
	).Scan(&senderPubkeyStr, &encSenderKey); err != nil {
		return "", 0, fmt.Errorf("fetch sender: %w", err)
	}

	var recipientPubkeyStr string
	if err := c.db.GetPool().QueryRow(ctx,
		"SELECT solana_pubkey FROM users WHERE id = $1", recipientID,
	).Scan(&recipientPubkeyStr); err != nil {
		return "", 0, fmt.Errorf("fetch recipient: %w", err)
	}

	senderPrivKey, err := decodeStoredPrivKey(encSenderKey)
	if err != nil {
		return "", 0, fmt.Errorf("decode sender key: %w", err)
	}

	senderPubkey := solana.MustPublicKeyFromBase58(senderPubkeyStr)
	recipientPubkey := solana.MustPublicKeyFromBase58(recipientPubkeyStr)

	// Cryptographically random nonce — uniquely identifies the escrow PDA.
	nonce, err := randomU64()
	if err != nil {
		return "", 0, fmt.Errorf("generate nonce: %w", err)
	}
	nonceBytes := le64(nonce)

	// Derive on-chain PDAs.
	escrowPDA, err := deriveEscrowPDA(c.programID, senderPubkey, recipientPubkey, nonceBytes)
	if err != nil {
		return "", 0, err
	}
	vaultPDA, err := deriveVaultPDA(c.programID, senderPubkey, recipientPubkey, nonceBytes)
	if err != nil {
		return "", 0, err
	}

	// Derive Associated Token Accounts.
	senderATA, err := findATA(senderPubkey, c.usdtMint)
	if err != nil {
		return "", 0, fmt.Errorf("derive sender ATA: %w", err)
	}
	feeTreasuryATA, err := findATA(c.treasuryWallet, c.usdtMint)
	if err != nil {
		return "", 0, fmt.Errorf("derive treasury ATA: %w", err)
	}

	// Instruction data: disc(8) || amount_le8(8) || nonce_le8(8) = 24 bytes.
	// The amount passed to the program is the total (net + fees) in USDT
	// micro-units (6 decimal places).
	totalMicro := uint64((netAmount + fees) * 1_000_000)
	instData := make([]byte, 24)
	copy(instData[:8], discInitiateTransfer[:])
	putLE64(instData[8:], totalMicro)
	putLE64(instData[16:], nonce)

	// Accounts in the order defined by the Rust InitiateTransfer context.
	accounts := []*solana.AccountMeta{
		solana.NewAccountMeta(senderPubkey, true, true),      // sender         – mut, signer
		solana.NewAccountMeta(senderATA, true, false),        // sender_token_account – mut
		solana.NewAccountMeta(recipientPubkey, false, false), // recipient       – readonly
		solana.NewAccountMeta(feeTreasuryATA, true, false),   // fee_treasury    – mut
		solana.NewAccountMeta(c.usdtMint, false, false),      // mint            – readonly
		solana.NewAccountMeta(escrowPDA, true, false),        // escrow_account  – mut (init)
		solana.NewAccountMeta(vaultPDA, true, false),         // escrow_vault    – mut (init)
		solana.NewAccountMeta(splTokenProgram, false, false), // token_program
		solana.NewAccountMeta(solana.SystemProgramID, false, false), // system_program
		solana.NewAccountMeta(solana.SysVarRentPubkey, false, false), // rent
	}

	inst := solana.NewInstruction(c.programID, accounts, instData)
	txSig, err := c.buildAndSendTx(ctx, senderPrivKey, inst)
	if err != nil {
		return "", 0, fmt.Errorf("submit initiate_transfer: %w", err)
	}
	return txSig, nonce, nil
}

// ClaimTransfer submits a claim_transfer instruction so the recipient receives
// the escrowed tokens (minus the 1 % fee sent to the treasury).
func (c *Client) ClaimTransfer(txHash string) error {
	if err := c.requireMintAndTreasury(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch transfer metadata needed to reconstruct the PDAs.
	var senderID, recipientID uint
	var nonce uint64
	if err := c.db.GetPool().QueryRow(ctx,
		`SELECT sender_id, recipient_id, COALESCE(escrow_nonce, 0)
		   FROM transfers WHERE solana_tx_hash = $1`, txHash,
	).Scan(&senderID, &recipientID, &nonce); err != nil {
		return fmt.Errorf("fetch transfer: %w", err)
	}

	var senderPubkeyStr string
	if err := c.db.GetPool().QueryRow(ctx,
		"SELECT solana_pubkey FROM users WHERE id = $1", senderID,
	).Scan(&senderPubkeyStr); err != nil {
		return fmt.Errorf("fetch sender pubkey: %w", err)
	}

	var recipientPubkeyStr, encRecipientKey string
	if err := c.db.GetPool().QueryRow(ctx,
		"SELECT solana_pubkey, encrypted_priv_key FROM users WHERE id = $1", recipientID,
	).Scan(&recipientPubkeyStr, &encRecipientKey); err != nil {
		return fmt.Errorf("fetch recipient: %w", err)
	}

	recipientPrivKey, err := decodeStoredPrivKey(encRecipientKey)
	if err != nil {
		return fmt.Errorf("decode recipient key: %w", err)
	}

	senderPubkey := solana.MustPublicKeyFromBase58(senderPubkeyStr)
	recipientPubkey := solana.MustPublicKeyFromBase58(recipientPubkeyStr)
	nonceBytes := le64(nonce)

	escrowPDA, err := deriveEscrowPDA(c.programID, senderPubkey, recipientPubkey, nonceBytes)
	if err != nil {
		return err
	}
	vaultPDA, err := deriveVaultPDA(c.programID, senderPubkey, recipientPubkey, nonceBytes)
	if err != nil {
		return err
	}

	recipientATA, err := findATA(recipientPubkey, c.usdtMint)
	if err != nil {
		return fmt.Errorf("derive recipient ATA: %w", err)
	}
	feeTreasuryATA, err := findATA(c.treasuryWallet, c.usdtMint)
	if err != nil {
		return fmt.Errorf("derive treasury ATA: %w", err)
	}

	// Instruction data: disc(8) || nonce_le8(8) = 16 bytes.
	instData := make([]byte, 16)
	copy(instData[:8], discClaimTransfer[:])
	putLE64(instData[8:], nonce)

	// Accounts in the order defined by the Rust ClaimTransfer context.
	accounts := []*solana.AccountMeta{
		solana.NewAccountMeta(recipientPubkey, true, true),   // recipient              – mut, signer
		solana.NewAccountMeta(recipientATA, true, false),     // recipient_token_account – mut
		solana.NewAccountMeta(senderPubkey, false, false),    // sender                 – readonly
		solana.NewAccountMeta(escrowPDA, true, false),        // escrow_account         – mut
		solana.NewAccountMeta(vaultPDA, true, false),         // escrow_vault           – mut
		solana.NewAccountMeta(feeTreasuryATA, true, false),   // fee_treasury           – mut
		solana.NewAccountMeta(c.usdtMint, false, false),      // mint                   – readonly
		solana.NewAccountMeta(splTokenProgram, false, false), // token_program
	}

	inst := solana.NewInstruction(c.programID, accounts, instData)
	_, err = c.buildAndSendTx(ctx, recipientPrivKey, inst)
	return err
}

// RefundTransfer submits a refund_transfer instruction so the sender recovers
// the full escrowed amount after the 7-day expiry window has passed.
func (c *Client) RefundTransfer(txHash string) error {
	if err := c.requireMintAndTreasury(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var senderID, recipientID uint
	var nonce uint64
	if err := c.db.GetPool().QueryRow(ctx,
		`SELECT sender_id, recipient_id, COALESCE(escrow_nonce, 0)
		   FROM transfers WHERE solana_tx_hash = $1`, txHash,
	).Scan(&senderID, &recipientID, &nonce); err != nil {
		return fmt.Errorf("fetch transfer: %w", err)
	}

	var senderPubkeyStr, encSenderKey string
	if err := c.db.GetPool().QueryRow(ctx,
		"SELECT solana_pubkey, encrypted_priv_key FROM users WHERE id = $1", senderID,
	).Scan(&senderPubkeyStr, &encSenderKey); err != nil {
		return fmt.Errorf("fetch sender: %w", err)
	}

	var recipientPubkeyStr string
	if err := c.db.GetPool().QueryRow(ctx,
		"SELECT solana_pubkey FROM users WHERE id = $1", recipientID,
	).Scan(&recipientPubkeyStr); err != nil {
		return fmt.Errorf("fetch recipient pubkey: %w", err)
	}

	senderPrivKey, err := decodeStoredPrivKey(encSenderKey)
	if err != nil {
		return fmt.Errorf("decode sender key: %w", err)
	}

	senderPubkey := solana.MustPublicKeyFromBase58(senderPubkeyStr)
	recipientPubkey := solana.MustPublicKeyFromBase58(recipientPubkeyStr)
	nonceBytes := le64(nonce)

	escrowPDA, err := deriveEscrowPDA(c.programID, senderPubkey, recipientPubkey, nonceBytes)
	if err != nil {
		return err
	}
	vaultPDA, err := deriveVaultPDA(c.programID, senderPubkey, recipientPubkey, nonceBytes)
	if err != nil {
		return err
	}

	senderATA, err := findATA(senderPubkey, c.usdtMint)
	if err != nil {
		return fmt.Errorf("derive sender ATA: %w", err)
	}

	// Instruction data: disc(8) || nonce_le8(8) = 16 bytes.
	instData := make([]byte, 16)
	copy(instData[:8], discRefundTransfer[:])
	putLE64(instData[8:], nonce)

	// Accounts in the order defined by the Rust RefundTransfer context.
	accounts := []*solana.AccountMeta{
		solana.NewAccountMeta(senderPubkey, true, true),      // sender               – mut, signer
		solana.NewAccountMeta(senderATA, true, false),        // sender_token_account – mut
		solana.NewAccountMeta(recipientPubkey, false, false), // recipient            – readonly
		solana.NewAccountMeta(escrowPDA, true, false),        // escrow_account       – mut
		solana.NewAccountMeta(vaultPDA, true, false),         // escrow_vault         – mut
		solana.NewAccountMeta(c.usdtMint, false, false),      // mint                 – readonly
		solana.NewAccountMeta(splTokenProgram, false, false), // token_program
	}

	inst := solana.NewInstruction(c.programID, accounts, instData)
	_, err = c.buildAndSendTx(ctx, senderPrivKey, inst)
	return err
}

// GetTokenBalance returns the USDT balance for the given Solana public-key string.
func (c *Client) GetTokenBalance(pubkey string) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pk, err := solana.PublicKeyFromBase58(pubkey)
	if err != nil {
		return 0, fmt.Errorf("invalid public key %q: %w", pubkey, err)
	}

	res, err := c.rpc.GetTokenAccountBalance(ctx, pk, rpc.CommitmentConfirmed)
	if err != nil {
		return 0, fmt.Errorf("GetTokenAccountBalance: %w", err)
	}
	if res == nil || res.Value == nil || res.Value.UiAmount == nil {
		return 0, nil
	}
	return *res.Value.UiAmount, nil
}

// GetTransactionStatus returns the database status of a transfer.
func (c *Client) GetTransactionStatus(txHash string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var status string
	if err := c.db.GetPool().QueryRow(ctx,
		"SELECT status FROM transfers WHERE solana_tx_hash = $1", txHash,
	).Scan(&status); err != nil {
		return "not_found", nil
	}
	return status, nil
}

// MarkTransferAsFailed sets a transfer's status to "failed".
func (c *Client) MarkTransferAsFailed(hash string) error  { return c.setStatus(hash, "failed") }

// MarkTransferAsExpired sets a transfer's status to "expired".
func (c *Client) MarkTransferAsExpired(hash string) error { return c.setStatus(hash, "expired") }

// MarkTransferAsCompleted sets a transfer's status to "completed".
func (c *Client) MarkTransferAsCompleted(hash string) error {
	return c.setStatus(hash, "completed")
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// requireMintAndTreasury returns an error when the client was not configured
// with a USDT mint and treasury wallet — both are required to build complete
// on-chain instructions.
func (c *Client) requireMintAndTreasury() error {
	if c.usdtMint.IsZero() {
		return fmt.Errorf("USDT_MINT_ADDRESS is not configured")
	}
	if c.treasuryWallet.IsZero() {
		return fmt.Errorf("TREASURY_PUBLIC_KEY is not configured")
	}
	return nil
}

// buildAndSendTx fetches the latest blockhash, builds and signs a transaction
// containing inst, submits it, and returns the signature string.
func (c *Client) buildAndSendTx(ctx context.Context, key solana.PrivateKey, inst solana.Instruction) (string, error) {
	latest, err := c.rpc.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("GetLatestBlockhash: %w", err)
	}

	tx, err := solana.NewTransaction(
		[]solana.Instruction{inst},
		latest.Value.Blockhash,
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

	sig, err := c.rpc.SendTransaction(ctx, tx)
	if err != nil {
		return "", fmt.Errorf("SendTransaction: %w", err)
	}
	return sig.String(), nil
}

// setStatus is a shared helper for the MarkTransferAs* methods.
func (c *Client) setStatus(hash, status string) error {
	_, err := c.db.GetPool().Exec(context.Background(),
		"UPDATE transfers SET status = $1 WHERE solana_tx_hash = $2", status, hash)
	return err
}

// ---------------------------------------------------------------------------
// PDA derivation
// ---------------------------------------------------------------------------

// deriveEscrowPDA computes the escrow account PDA.
// Seeds: ["diaspora-escrow", sender, recipient, nonce_le8]
func deriveEscrowPDA(programID, sender, recipient solana.PublicKey, nonceLE8 []byte) (solana.PublicKey, error) {
	addr, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("diaspora-escrow"), sender.Bytes(), recipient.Bytes(), nonceLE8},
		programID,
	)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("derive escrow PDA: %w", err)
	}
	return addr, nil
}

// deriveVaultPDA computes the escrow vault token account PDA.
// Seeds: ["diaspora-vault", sender, recipient, nonce_le8]
func deriveVaultPDA(programID, sender, recipient solana.PublicKey, nonceLE8 []byte) (solana.PublicKey, error) {
	addr, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("diaspora-vault"), sender.Bytes(), recipient.Bytes(), nonceLE8},
		programID,
	)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("derive vault PDA: %w", err)
	}
	return addr, nil
}

// findATA computes the Associated Token Account address for a wallet / mint pair.
// The ATA program seeds are: [wallet, token_program, mint].
func findATA(wallet, mint solana.PublicKey) (solana.PublicKey, error) {
	addr, _, err := solana.FindProgramAddress(
		[][]byte{wallet.Bytes(), splTokenProgram.Bytes(), mint.Bytes()},
		ataProgram,
	)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("derive ATA: %w", err)
	}
	return addr, nil
}

// ---------------------------------------------------------------------------
// Encoding helpers
// ---------------------------------------------------------------------------

// le64 encodes v as an 8-byte little-endian slice.
func le64(v uint64) []byte {
	b := make([]byte, 8)
	putLE64(b, v)
	return b
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

// decodeStoredPrivKey decodes a base64-encoded 64-byte Ed25519 private key.
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

// randomU64 returns a cryptographically random uint64.
func randomU64() (uint64, error) {
	max := new(big.Int).SetUint64(^uint64(0))
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0, err
	}
	return n.Uint64(), nil
}
