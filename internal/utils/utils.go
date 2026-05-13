package utils

import (
	"crypto/rand"

	"github.com/gagliardetto/solana-go"
)

func GenerateOTP() string { return rarandomDigits(6) }

func rarandomDigits(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return string(b)
}

func NewSolanaAddress() (string, solana.PrivateKey, error) {
	wallet := solana.NewWallet()
	return wallet.PublicKey().String(), wallet.PrivateKey, nil
}
