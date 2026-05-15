package utils

import (
	"crypto/rand"
	"strconv"

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

func ParseAmount(amount string) float64 {
	val, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return 0
	}
	return val
}
