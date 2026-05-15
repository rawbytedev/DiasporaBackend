package solana_test

import (
	"Diaspora/internal/mocks"
	"errors"
	"testing"
)

// ---------------------------------------------------------------------------
// InitiateTransfer
// ---------------------------------------------------------------------------

func TestMockInitiateTransfer_Success(t *testing.T) {
	client := mocks.NewMockSolanaClient()

	hash, err := client.InitiateTransfer(1, 2, 99.0, 1.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty tx hash")
	}

	history := client.GetTransferHistory()
	rec, ok := history[hash]
	if !ok {
		t.Fatalf("transfer %q not found in history", hash)
	}
	if rec.SenderID != 1 || rec.RecipientID != 2 {
		t.Errorf("IDs mismatch: got sender=%d recipient=%d", rec.SenderID, rec.RecipientID)
	}
	if rec.NetAmount != 99.0 {
		t.Errorf("net amount: want 99.0, got %f", rec.NetAmount)
	}
	if rec.Status != "pending" {
		t.Errorf("status: want pending, got %s", rec.Status)
	}
}

func TestMockInitiateTransfer_Error(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	client.SetInitiateError(errors.New("insufficient funds"))

	_, err := client.InitiateTransfer(1, 2, 99.0, 1.0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "insufficient funds" {
		t.Errorf("error message: want 'insufficient funds', got %q", err.Error())
	}
}

func TestMockInitiateTransfer_UniqueHashes(t *testing.T) {
	client := mocks.NewMockSolanaClient()

	hash1, _ := client.InitiateTransfer(1, 2, 50.0, 0.5)
	hash2, _ := client.InitiateTransfer(1, 2, 50.0, 0.5)

	if hash1 == hash2 {
		t.Errorf("expected unique hashes, got same: %s", hash1)
	}
}

// ---------------------------------------------------------------------------
// ClaimTransfer
// ---------------------------------------------------------------------------

func TestMockClaimTransfer_Success(t *testing.T) {
	client := mocks.NewMockSolanaClient()

	hash, err := client.InitiateTransfer(1, 2, 99.0, 1.0)
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}

	if err := client.ClaimTransfer(hash); err != nil {
		t.Fatalf("claim: %v", err)
	}

	status, err := client.GetTransactionStatus(hash)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != "claimed" {
		t.Errorf("status: want claimed, got %s", status)
	}
}

func TestMockClaimTransfer_Error(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	client.SetClaimError(errors.New("claim rejected"))

	hash, _ := client.InitiateTransfer(1, 2, 99.0, 1.0)
	if err := client.ClaimTransfer(hash); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMockClaimTransfer_NotFound(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	err := client.ClaimTransfer("nonexistent_hash")
	if err == nil {
		t.Fatal("expected error for unknown hash, got nil")
	}
}

// ---------------------------------------------------------------------------
// RefundTransfer
// ---------------------------------------------------------------------------

func TestMockRefundTransfer_Success(t *testing.T) {
	client := mocks.NewMockSolanaClient()

	hash, _ := client.InitiateTransfer(1, 2, 99.0, 1.0)

	if err := client.RefundTransfer(hash); err != nil {
		t.Fatalf("refund: %v", err)
	}

	status, _ := client.GetTransactionStatus(hash)
	if status != "refunded" {
		t.Errorf("status: want refunded, got %s", status)
	}
}

func TestMockRefundTransfer_Error(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	client.SetRefundError(errors.New("refund not available"))

	hash, _ := client.InitiateTransfer(1, 2, 99.0, 1.0)
	if err := client.RefundTransfer(hash); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMockRefundTransfer_NotFound(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	err := client.RefundTransfer("does_not_exist")
	if err == nil {
		t.Fatal("expected error for unknown hash, got nil")
	}
}

// ---------------------------------------------------------------------------
// GetTokenBalance
// ---------------------------------------------------------------------------

func TestMockGetTokenBalance_Default(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	balance, err := client.GetTokenBalance("any_pubkey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if balance != 1000.0 {
		t.Errorf("balance: want 1000.0, got %f", balance)
	}
}

func TestMockGetTokenBalance_Custom(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	client.SetMockBalance(42.5)

	balance, err := client.GetTokenBalance("pubkey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if balance != 42.5 {
		t.Errorf("balance: want 42.5, got %f", balance)
	}
}

func TestMockGetTokenBalance_Error(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	client.SetBalanceError(errors.New("node unavailable"))

	_, err := client.GetTokenBalance("pubkey")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// GetTransactionStatus
// ---------------------------------------------------------------------------

func TestMockGetTransactionStatus_NotFound(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	status, err := client.GetTransactionStatus("unknown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "not_found" {
		t.Errorf("status: want not_found, got %s", status)
	}
}

func TestMockGetTransactionStatus_Lifecycle(t *testing.T) {
	client := mocks.NewMockSolanaClient()

	hash, _ := client.InitiateTransfer(3, 4, 200.0, 2.0)

	status, _ := client.GetTransactionStatus(hash)
	if status != "pending" {
		t.Errorf("after initiate: want pending, got %s", status)
	}

	client.ClaimTransfer(hash)

	status, _ = client.GetTransactionStatus(hash)
	if status != "claimed" {
		t.Errorf("after claim: want claimed, got %s", status)
	}
}

// ---------------------------------------------------------------------------
// Concurrency safety
// ---------------------------------------------------------------------------

func TestMockConcurrencySafe(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	done := make(chan struct{}, 10)

	for i := 0; i < 5; i++ {
		go func(id uint) {
			client.InitiateTransfer(id, id+1, float64(id)*10, float64(id)*0.1)
			done <- struct{}{}
		}(uint(i + 1))
	}

	for i := 0; i < 5; i++ {
		go func() {
			client.GetTokenBalance("some_pubkey")
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
