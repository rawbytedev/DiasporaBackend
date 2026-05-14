package mocks

import (
	"sync"
)

// MockMobileMoneyClient mocks the Mobile Money client
type MockMobileMoneyClient struct {
	mu                  sync.Mutex
	sendMoneyError      error
	statusError         error
	lastPhone           string
	lastAmount          float64
	transactionHistory  map[string]TransactionRecord
	nextTransactionID   int
}

type TransactionRecord struct {
	Phone  string
	Amount float64
	Status string
}

// NewMockMobileMoneyClient creates a new mock Mobile Money client
func NewMockMobileMoneyClient() *MockMobileMoneyClient {
	return &MockMobileMoneyClient{
		transactionHistory: make(map[string]TransactionRecord),
		nextTransactionID:  1000,
	}
}

// SendMoney mocks sending money via mobile money
func (m *MockMobileMoneyClient) SendMoney(phone string, amount float64, provider string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sendMoneyError != nil {
		return "", m.sendMoneyError
	}

	txID := "MM_" + string(rune(m.nextTransactionID))
	m.nextTransactionID++
	m.lastPhone = phone
	m.lastAmount = amount
	m.transactionHistory[txID] = TransactionRecord{
		Phone:  phone,
		Amount: amount,
		Status: "pending",
	}
	return txID, nil
}

// CheckTransactionStatus mocks checking transaction status
func (m *MockMobileMoneyClient) CheckTransactionStatus(txID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.statusError != nil {
		return "", m.statusError
	}

	if record, ok := m.transactionHistory[txID]; ok {
		return record.Status, nil
	}
	return "not_found", nil
}

// SetSendMoneyError sets an error to be returned on SendMoney
func (m *MockMobileMoneyClient) SetSendMoneyError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendMoneyError = err
}

// SetStatusError sets an error to be returned on CheckTransactionStatus
func (m *MockMobileMoneyClient) SetStatusError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusError = err
}

// SetTransactionStatus sets the status of a transaction
func (m *MockMobileMoneyClient) SetTransactionStatus(txID string, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if record, ok := m.transactionHistory[txID]; ok {
		record.Status = status
		m.transactionHistory[txID] = record
	}
}

// GetTransactionHistory returns all transactions
func (m *MockMobileMoneyClient) GetTransactionHistory() map[string]TransactionRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	history := make(map[string]TransactionRecord)
	for k, v := range m.transactionHistory {
		history[k] = v
	}
	return history
}

// MockKYCClient mocks the KYC client
type MockKYCClient struct {
	mu          sync.Mutex
	verifyError error
	attachError error
	verifiedIDs map[uint]bool
}

// NewMockKYCClient creates a new mock KYC client
func NewMockKYCClient() *MockKYCClient {
	return &MockKYCClient{
		verifiedIDs: make(map[uint]bool),
	}
}

// Verify mocks verifying a user's KYC status
func (m *MockKYCClient) Verify(userID uint) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.verifyError != nil {
		return false, m.verifyError
	}

	return m.verifiedIDs[userID], nil
}

// AttachInfo mocks attaching KYC information
func (m *MockKYCClient) AttachInfo(userID uint, info map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.attachError != nil {
		return m.attachError
	}

	m.verifiedIDs[userID] = true
	return nil
}

// SetVerifyError sets an error to be returned on Verify
func (m *MockKYCClient) SetVerifyError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifyError = err
}

// SetAttachError sets an error to be returned on AttachInfo
func (m *MockKYCClient) SetAttachError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attachError = err
}

// MarkUserVerified marks a user as verified
func (m *MockKYCClient) MarkUserVerified(userID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifiedIDs[userID] = true
}

// MarkUserUnverified marks a user as unverified
func (m *MockKYCClient) MarkUserUnverified(userID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifiedIDs[userID] = false
}
