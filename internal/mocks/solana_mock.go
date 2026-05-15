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
	counter uint64

	initiateError error
	claimError    error
	refundError   error
	balanceError  error

	mockBalance     float64
	transferHistory map[string]TransferRecord
}

// TransferRecord is a snapshot of a single in-memory transfer stored by the mock.
type TransferRecord struct {
	SenderID    uint
	RecipientID uint
	NetAmount   float64
	Fees        float64
	Nonce       uint64
	Status      string // "pending" | "claimed" | "refunded"
}

// NewMockSolanaClient returns a MockSolanaClient with a default mock balance
// of 1 000 USDT and an empty transfer history.
func NewMockSolanaClient() *MockSolanaClient {
	return &MockSolanaClient{
		mockBalance:     1000.0,
		transferHistory: make(map[string]TransferRecord),
	}
}

// InitiateTransfer records the transfer and returns a deterministic fake hash
// plus a monotonically increasing nonce.
func (m *MockSolanaClient) InitiateTransfer(senderID, recipientID uint, netAmount, fees float64) (string, uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initiateError != nil {
		return "", 0, m.initiateError
	}

	n := atomic.AddUint64(&m.counter, 1)
	txHash := fmt.Sprintf("mock_tx_%d_s%d_r%d", n, senderID, recipientID)

	m.transferHistory[txHash] = TransferRecord{
		SenderID:    senderID,
		RecipientID: recipientID,
		NetAmount:   netAmount,
		Fees:        fees,
		Nonce:       n,
		Status:      "pending",
	}
	return txHash, n, nil
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
// Test helpers
// ---------------------------------------------------------------------------

func (m *MockSolanaClient) SetMockBalance(balance float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockBalance = balance
}

func (m *MockSolanaClient) SetInitiateError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initiateError = err
}

func (m *MockSolanaClient) SetClaimError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.claimError = err
}

func (m *MockSolanaClient) SetRefundError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refundError = err
}

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
