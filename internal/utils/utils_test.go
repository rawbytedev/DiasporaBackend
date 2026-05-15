package utils

import (
	"testing"
)

func TestParseAmount(t *testing.T) {
	if got := ParseAmount("42.5"); got != 42.5 {
		t.Fatalf("expected 42.5, got %f", got)
	}
	if got := ParseAmount("invalid"); got != 0 {
		t.Fatalf("expected 0 for invalid input, got %f", got)
	}
}

func TestGenerateOTP(t *testing.T) {
	otp := GenerateOTP()
	if len(otp) != 6 {
		t.Fatalf("expected 6 characters, got %d", len(otp))
	}
}

func TestNewSolanaAddress(t *testing.T) {
	pubkey, privKey, err := NewSolanaAddress()
	if err != nil {
		t.Fatalf("NewSolanaAddress returned error: %v", err)
	}
	if pubkey == "" {
		t.Fatal("expected non-empty public key")
	}
	if privKey.String() == "" {
		t.Fatal("expected non-empty private key")
	}
}
