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
	ID           uint    `gorm:"primaryKey"`
	SenderID     uint    `gorm:"index;not null"`
	RecipientID  uint    `gorm:"index;not null"`
	AmountUSDT   float64 `gorm:"type:decimal(20,6);not null"`
	FeesUSDT     float64 `gorm:"type:decimal(20,6);not null"`
	Status       string  `gorm:"type:varchar(20);default:'pending'"` // pending, claimed, refunded
	SolanaTxHash string  `gorm:"size:88;uniqueIndex"`
	CreatedAt    time.Time
	ExpiresAt    time.Time
	ClaimedAt    *time.Time
}

type Withdrawal struct {
	ID          uint    `gorm:"primaryKey"`
	UserID      uint    `gorm:"index;not null"`
	AmountFCFA  float64 `gorm:"type:decimal(20,0);not null"`
	PhoneNumber string  `gorm:"size:20;not null"`
	Provider    string  `gorm:"size:10"` // MTN, MOOV
	APITxID     string  `gorm:"size:100"`
	Status      string  `gorm:"default:'pending'"`
	CreatedAt   time.Time
}
