package mobilemoney

import "testing"

func TestNewClient(t *testing.T) {
	client := NewClient("https://api.example.com", "key", "secret")
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestSendMoneyAndCheckTransactionStatus(t *testing.T) {
	client := NewClient("https://api.example.com", "key", "secret")
	txID, err := client.SendMoney("+22912345678", 10.5, "mtn")
	if err != nil {
		t.Fatalf("SendMoney failed: %v", err)
	}
	if txID == "" {
		t.Fatal("expected non-empty transaction ID")
	}

	status, err := client.CheckTransactionStatus(txID)
	if err != nil {
		t.Fatalf("CheckTransactionStatus failed: %v", err)
	}
	if status != "success" {
		t.Fatalf("expected status success, got %q", status)
	}
}
