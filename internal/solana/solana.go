package solana

import (
	"Diaspora/internal/db"
	"context"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type Client struct {
	client *rpc.Client
	db     *db.PostgresDB
	admin  solana.PrivateKey
}

func (c *Client) InitiateTransfer(senderID uint, recipientID uint, netAmount float64, fees float64) (string, error) {
	var privateKey solana.PrivateKey
	dbTx, err := c.db.GetPool().Begin(context.Background())
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			dbTx.Rollback(context.Background())
		} else {
			dbTx.Commit(context.Background())
		}
	}()
	// Lock sender's row to prevent concurrent modifications
	err = dbTx.QueryRow(context.Background(), "SELECT encrypted_priv_key FROM users WHERE id = $1 FOR UPDATE", senderID).Scan(&privateKey)
	if err != nil {
		return "", err
	}
	// Lock recipient's row to prevent concurrent modifications
	var recipientPubKey solana.PublicKey
	err = dbTx.QueryRow(context.Background(), "SELECT solana_pubkey FROM users WHERE id = $1 FOR UPDATE", recipientID).Scan(&recipientPubKey)
	if err != nil {
		return "", err
	}
	// Update sender's balance
	err = dbTx.QueryRow(context.Background(), "UPDATE users SET balance_usdt = balance_usdt - $1 WHERE id = $2", netAmount+fees, senderID).Scan()
	if err != nil {
		return "", err
	}
	return c.transferTokens(privateKey, recipientPubKey, netAmount+fees)

}

func (c *Client) transferTokens(senderPrivKey solana.PrivateKey, recipientPubKey solana.PublicKey, amount float64) (string, error) {
	recentBlockhash, err := c.client.GetRecentBlockhash(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		return "", err
	}
	var mockProgramID solana.PublicKey
	code := []byte("mock transfer instruction data amount") // In a real implementation, this would be the actual instruction data for our smart contract
	_ = amount                                              // to avoid unused variable error, in a real implementation this would be encoded into the instruction data
	inst := solana.NewInstruction(mockProgramID, []*solana.AccountMeta{
		solana.NewAccountMeta(senderPrivKey.PublicKey(), true, false),
		solana.NewAccountMeta(recipientPubKey, false, true),
	}, code) // In a real implementation, this would be a call to our actual smart contract with the appropriate data
	tx, err := solana.NewTransaction(
		[]solana.Instruction{inst}, recentBlockhash.Value.Blockhash, solana.TransactionPayer(senderPrivKey.PublicKey()),
	)
	if err != nil {
		return "", err
	}
	// Sign the transaction with the sender's private key
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if senderPrivKey.PublicKey().Equals(key) {
			return &senderPrivKey
		}
		return nil
	})
	// Send the transaction
	res, err := c.client.SendTransaction(context.Background(), tx)
	if err != nil {
		return "", err
	}
	return res.String(), nil
}

// In a real implementation, we would also have functions to handle incoming transactions, check their status, and update the database accordingly. For simplicity, we'll just have a function to mark a transfer as failed or completed based on the transaction hash.
func (c *Client) MarkTransferAsFailed(hash string) error {
	_, err := c.db.GetPool().Exec(context.Background(), "UPDATE transfers SET status = 'failed' WHERE solana_tx_hash = $1", hash)
	return err
}

func (c *Client) MarkTransferAsExpired(hash string) error {
	_, err := c.db.GetPool().Exec(context.Background(), "UPDATE transfers SET status = 'expired' WHERE solana_tx_hash = $1", hash)
	return err
}

func (c *Client) MarkTransferAsCompleted(hash string) error {
	_, err := c.db.GetPool().Exec(context.Background(), "UPDATE transfers SET status = 'completed' WHERE solana_tx_hash = $1", hash)
	return err
}

func (c *Client) GetTransferStatus(hash string) (string, error) {
	var status string
	err := c.db.GetPool().QueryRow(context.Background(), "SELECT status FROM transfers WHERE solana_tx_hash = $1", hash).Scan(&status)
	return status, err
}

func (c *Client) ClaimTransfer(hash string) error {
	var userID uint
	status, err := c.GetTransactionStatus(hash)
	if err != nil {
		return err
	}
	c.db.GetPool().QueryRow(context.Background(), "SELECT recipient_id FROM transfers WHERE solana_tx_hash = $1", hash).Scan(&userID)
	if status != "confirmed" {
		return nil // pas encore confirmé, on réessaiera plus tard
	}
	// For now funds are sent directly to servers wallet, so no need to do anything here. In a real implementation, we would now transfer the funds from the server wallet to their corresponding bridge wallet or conversion ramp.
	c.db.GetPool().QueryRow(context.Background(), "UPDATE transfers SET status = 'completed' WHERE solana_tx_hash = $1 AND recipient_id = $2", hash, userID)

	c.db.GetPool().QueryRow(context.Background(), "UPDATE users SET balance_usdt = balance_usdt + (SELECT amount_usdt FROM transfers WHERE solana_tx_hash = $1) WHERE id = $2", hash, userID)

	return nil
}

func (c *Client) GetTransactionStatus(txHash string) (string, error) {
	res, err := c.client.GetConfirmedTransaction(context.Background(), solana.MustSignatureFromBase58(txHash))
	if err != nil {
		return "", err
	}
	return string(res.Transaction.GetRawJSON()), nil
}

func (c *Client) GetTokenBalance(pubkey string) (float64, error) {
	res, err := c.client.GetTokenAccountBalance(context.Background(), solana.MustPublicKeyFromBase58(pubkey), rpc.CommitmentConfirmed)
	return *res.Value.UiAmount, err
}

func NewClient(endpoint string, db *db.PostgresDB, admin string) (*Client, error) {
	solanaAdmin, err := solana.PrivateKeyFromBase58(admin)
	if err != nil {
		return nil, err
	}
	return &Client{
		client: rpc.New(endpoint),
		db:     db,
		admin:  solanaAdmin,
	}, nil
}

