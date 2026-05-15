package models

import (
	"time"

	"github.com/gagliardetto/solana-go"
)

type User struct {
	ID               uint              `gorm:"primaryKey"`
	PhoneNumber      string            `gorm:"uniqueIndex;size:20;not null"`
	SolanaPubkey     string            `gorm:"size:44;not null"`   // base58
	EncryptedPrivKey string            `gorm:"type:text;not null"` // chiffré AES
	MockPrivKey      solana.PrivateKey `gorm:"-"`                  // non stocké en DB, utilisé pour les tests
	Name             string            `gorm:"size:100"`
	StateVersion     int               `gorm:"default:1"` // incrémenté à chaque modification
	CreatedAt        time.Time
}

type Transfer struct {
	ID           uint    `gorm:"primaryKey"                         json:"id"`
	SenderID     uint    `gorm:"index;not null"                     json:"sender_id"`
	RecipientID  uint    `gorm:"index;not null"                     json:"recipient_id"`
	AmountUSDT   float64 `gorm:"type:decimal(20,6);not null"        json:"amount_usdt"`
	FeesUSDT     float64 `gorm:"type:decimal(20,6);not null"        json:"fees_usdt"`
	Status       string  `gorm:"type:varchar(20);default:'pending'" json:"status"`
	SolanaTxHash string  `gorm:"size:88;uniqueIndex"                json:"solana_tx_hash"`
	// EscrowNonce is the random u64 PDA seed generated during InitiateTransfer.
	// Must be persisted so ClaimTransfer / RefundTransfer can reconstruct the
	// same on-chain escrow and vault PDAs.
	EscrowNonce uint64     `gorm:"column:escrow_nonce" json:"escrow_nonce"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	ClaimedAt   *time.Time `json:"claimed_at,omitempty"`
}

type Withdrawal struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"index;not null" json:"user_id"`
	AmountFCFA  float64   `gorm:"type:decimal(20,0);not null" json:"amount_fcfa"`
	PhoneNumber string    `gorm:"size:20;not null" json:"phone_number"`
	Provider    string    `gorm:"size:10" json:"provider"` // MTN, MOOV
	APITxID     string    `gorm:"size:100" json:"api_tx_id"`
	Status      string    `gorm:"default:'pending'" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}
