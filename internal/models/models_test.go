package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTransferJSONMarshall(t *testing.T) {
	tf := Transfer{
		ID:           1,
		SenderID:     10,
		RecipientID:  20,
		AmountUSDT:   100.5,
		FeesUSDT:     1.0,
		Status:       "pending",
		SolanaTxHash: "test_hash",
		EscrowNonce:  12345,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	data, err := json.Marshal(tf)
	if err != nil {
		t.Fatalf("failed to marshal transfer: %v", err)
	}
	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"escrow_nonce"`) {
		t.Fatalf("expected escrow_nonce field to be present in JSON: %s", jsonStr)
	}
	if strings.Contains(jsonStr, `"claimed_at"`) {
		t.Fatalf("expected claimed_at field to be omitted when nil: %s", jsonStr)
	}
}

func TestWithdrawalJSONMarshal(t *testing.T) {
	wd := Withdrawal{
		ID:          1,
		UserID:      10,
		AmountFCFA:  5000,
		PhoneNumber: "+22912345678",
		Provider:    "mtn",
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	data, err := json.Marshal(wd)
	if err != nil {
		t.Fatalf("failed to marshal withdrawal: %v", err)
	}
	if !strings.Contains(string(data), `"amount_fcfa"`) {
		t.Fatal("expected amount_fcfa field to be present in JSON")
	}
}
