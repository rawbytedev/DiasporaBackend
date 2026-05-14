package mocks

import (
	"Diaspora/internal/db"
	"sync"
)

// MockSolanaClient mocks the Solana blockchain client for testing
type MockSolanaClient struct {
	mu              sync.Mutex
	initiateError   error
	claimError      error
	refundError     error
	balanceError    error
	mockBalance     float64
	lastTxHash      string
	transferHistory map[string]TransferRecord
}

type TransferRecord struct {
	SenderID    uint
	RecipientID uint
	Amount      float64
	Status      string
}

// NewMockSolanaClient creates a new mock Solana client
func NewMockSolanaClient() *MockSolanaClient {
	return &MockSolanaClient{
		mockBalance:     1000.0, // default mock balance
		transferHistory: make(map[string]TransferRecord),
	}
}

// InitiateTransfer mocks initiating a transfer
func (m *MockSolanaClient) InitiateTransfer(senderID uint, recipientID uint, netAmount float64, fees float64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initiateError != nil {
		return "", m.initiateError
	}

	txHash := "mock_tx_" + string(rune(senderID)) + "_" + string(rune(recipientID))
	m.lastTxHash = txHash
	m.transferHistory[txHash] = TransferRecord{
		SenderID:    senderID,
		RecipientID: recipientID,
		Amount:      netAmount,
		Status:      "pending",
	}
	return txHash, nil
}

// ClaimTransfer mocks claiming a transfer
func (m *MockSolanaClient) ClaimTransfer(hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.claimError != nil {
		return m.claimError
	}

	if record, ok := m.transferHistory[hash]; ok {
		record.Status = "claimed"
		m.transferHistory[hash] = record
	}
	return nil
}

// RefundTransfer mocks refunding a transfer
func (m *MockSolanaClient) RefundTransfer(hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.refundError != nil {
		return m.refundError
	}

	if record, ok := m.transferHistory[hash]; ok {
		record.Status = "refunded"
		m.transferHistory[hash] = record
	}
	return nil
}

// GetTokenBalance mocks getting token balance
func (m *MockSolanaClient) GetTokenBalance(pubkey string) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.balanceError != nil {
		return 0, m.balanceError
	}
	return m.mockBalance, nil
}

// GetTransactionStatus mocks getting transaction status
func (m *MockSolanaClient) GetTransactionStatus(txHash string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if record, ok := m.transferHistory[txHash]; ok {
		return record.Status, nil
	}
	return "not_found", nil
}

// SetMockBalance sets the mock balance for testing
func (m *MockSolanaClient) SetMockBalance(balance float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockBalance = balance
}

// SetInitiateError sets an error to be returned on InitiateTransfer
func (m *MockSolanaClient) SetInitiateError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initiateError = err
}

// SetClaimError sets an error to be returned on ClaimTransfer
func (m *MockSolanaClient) SetClaimError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.claimError = err
}

// SetRefundError sets an error to be returned on RefundTransfer
func (m *MockSolanaClient) SetRefundError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refundError = err
}

// SetBalanceError sets an error to be returned on GetTokenBalance
func (m *MockSolanaClient) SetBalanceError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balanceError = err
}

// NewClient mock initializer
func NewClient(endpoint string, db *db.PostgresDB, admin string) *MockSolanaClient {
	return NewMockSolanaClient()
}

// GetTransferHistory returns the history of all transfers
func (m *MockSolanaClient) GetTransferHistory() map[string]TransferRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	history := make(map[string]TransferRecord)
	for k, v := range m.transferHistory {
		history[k] = v
	}
	return history
}
