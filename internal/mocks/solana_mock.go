// Package mocks provides in-memory test doubles for external clients.
// All mocks implement the same interfaces as their real counterparts so they
// can be injected anywhere an interface is expected.
package mocks

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// MockSolanaClient is a thread-safe, in-memory implementation of
// solana.ClientInterface.  It records every call and allows tests to inject
// specific error conditions or inspect call history.
type MockSolanaClient struct {
	mu      sync.Mutex
	counter uint64 // used for deterministic hash generation

	initiateError error
	claimError    error
	refundError   error
	balanceError  error

	mockBalance     float64
	transferHistory map[string]TransferRecord
}

// TransferRecord is a snapshot of a single on-chain transfer stored by the mock.
type TransferRecord struct {
	SenderID    uint
	RecipientID uint
	NetAmount   float64
	Fees        float64
	Status      string // "pending" | "claimed" | "refunded"
}

// NewMockSolanaClient returns a MockSolanaClient with a default mock balance of
// 1 000 USDT and an empty transfer history.
func NewMockSolanaClient() *MockSolanaClient {
	return &MockSolanaClient{
		mockBalance:     1000.0,
		transferHistory: make(map[string]TransferRecord),
	}
}

// InitiateTransfer records the transfer and returns a deterministic fake hash.
func (m *MockSolanaClient) InitiateTransfer(senderID uint, recipientID uint, netAmount float64, fees float64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initiateError != nil {
		return "", m.initiateError
	}

	n := atomic.AddUint64(&m.counter, 1)
	txHash := fmt.Sprintf("mock_tx_%d_s%d_r%d", n, senderID, recipientID)

	m.transferHistory[txHash] = TransferRecord{
		SenderID:    senderID,
		RecipientID: recipientID,
		NetAmount:   netAmount,
		Fees:        fees,
		Status:      "pending",
	}
	return txHash, nil
}

// ClaimTransfer marks the transfer as claimed.
func (m *MockSolanaClient) ClaimTransfer(hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.claimError != nil {
		return m.claimError
	}

	rec, ok := m.transferHistory[hash]
	if !ok {
		return fmt.Errorf("transfer not found: %s", hash)
	}
	rec.Status = "claimed"
	m.transferHistory[hash] = rec
	return nil
}

// RefundTransfer marks the transfer as refunded.
func (m *MockSolanaClient) RefundTransfer(hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.refundError != nil {
		return m.refundError
	}

	rec, ok := m.transferHistory[hash]
	if !ok {
		return fmt.Errorf("transfer not found: %s", hash)
	}
	rec.Status = "refunded"
	m.transferHistory[hash] = rec
	return nil
}

// GetTokenBalance returns the configured mock balance.
func (m *MockSolanaClient) GetTokenBalance(_ string) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.balanceError != nil {
		return 0, m.balanceError
	}
	return m.mockBalance, nil
}

// GetTransactionStatus returns the status of a recorded transfer.
func (m *MockSolanaClient) GetTransactionStatus(txHash string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rec, ok := m.transferHistory[txHash]; ok {
		return rec.Status, nil
	}
	return "not_found", nil
}

// ---------------------------------------------------------------------------
// Test helpers (not part of the interface)
// ---------------------------------------------------------------------------

// SetMockBalance changes the balance returned by GetTokenBalance.
func (m *MockSolanaClient) SetMockBalance(balance float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockBalance = balance
}

// SetInitiateError causes the next InitiateTransfer call to return err.
// Pass nil to clear the error.
func (m *MockSolanaClient) SetInitiateError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initiateError = err
}

// SetClaimError causes the next ClaimTransfer call to return err.
func (m *MockSolanaClient) SetClaimError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.claimError = err
}

// SetRefundError causes the next RefundTransfer call to return err.
func (m *MockSolanaClient) SetRefundError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refundError = err
}

// SetBalanceError causes the next GetTokenBalance call to return err.
func (m *MockSolanaClient) SetBalanceError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balanceError = err
}

// GetTransferHistory returns a snapshot of all recorded transfers.
func (m *MockSolanaClient) GetTransferHistory() map[string]TransferRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap := make(map[string]TransferRecord, len(m.transferHistory))
	for k, v := range m.transferHistory {
		snap[k] = v
	}
	return snap
}
