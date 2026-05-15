// Package solana provides the interface and implementation for
// interacting with the Solana blockchain and the DiasporaConnect smart contract.
package solana

// ClientInterface defines all blockchain operations required by the application.
// Both the real Client and the mock implementations satisfy this interface,
// enabling full handler-level unit tests without a live Solana node.
type ClientInterface interface {
	// InitiateTransfer locks tokens in the on-chain escrow PDA.
	// Returns the base-58 transaction signature used as a stable record ID.
	InitiateTransfer(senderID uint, recipientID uint, netAmount float64, fees float64) (string, error)

	// ClaimTransfer releases escrowed tokens to the recipient.
	ClaimTransfer(txHash string) error

	// RefundTransfer returns escrowed tokens to the sender after expiry.
	RefundTransfer(txHash string) error

	// GetTokenBalance returns the USDT token balance for a Solana public key.
	GetTokenBalance(pubkey string) (float64, error)

	// GetTransactionStatus returns the on-chain status of a transfer ("pending",
	// "confirmed", "not_found", etc.).
	GetTransactionStatus(txHash string) (string, error)
}
