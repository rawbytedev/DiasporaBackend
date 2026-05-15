// Package solana provides the interface and implementation for
// interacting with the Solana blockchain and the DiasporaConnect smart contract.
package solana

// ClientInterface defines all blockchain operations required by the application.
// Both the real Client and the mock implementations satisfy this interface,
// enabling full handler-level unit tests without a live Solana node.
type ClientInterface interface {
	// InitiateTransfer locks tokens in the on-chain escrow PDA.
	// Returns the base-58 transaction signature and the random nonce used to
	// derive the escrow PDA — the caller must persist the nonce so that
	// ClaimTransfer and RefundTransfer can reconstruct the same PDA.
	InitiateTransfer(senderID uint, recipientID uint, netAmount float64, fees float64) (txHash string, nonce uint64, err error)

	// ClaimTransfer releases escrowed tokens to the recipient.
	// The transfer identified by txHash must be in "pending" status and must
	// not have expired.
	ClaimTransfer(txHash string) error

	// RefundTransfer returns escrowed tokens to the sender after the 7-day expiry.
	RefundTransfer(txHash string) error

	// GetTokenBalance returns the USDT token balance for a Solana public key.
	GetTokenBalance(pubkey string) (float64, error)

	// GetTransactionStatus returns the database status of a transfer
	// ("pending", "claimed", "refunded", "not_found").
	GetTransactionStatus(txHash string) (string, error)
}
