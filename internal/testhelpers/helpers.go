package testhelpers

import (
	"Diaspora/internal/cache"
	"Diaspora/internal/db"
	"Diaspora/internal/models"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
)

const (
	TestDBDSN = "host=localhost port=5432 user=AdminDias password=Admin dbname=diaspora_test sslmode=disable"
)

type TestDB struct {
	*db.PostgresDB
	T *testing.T
}

type TestContext struct {
	DB    *TestDB
	Cache *cache.CacheStore
	T     *testing.T
}

// InitializeTestDatabase creates all necessary tables for tests
func InitializeTestDatabase(ctx context.Context, postgresDB *db.PostgresDB) error {
	// Create users table
	usersSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			password VARCHAR(255) NOT NULL,
			phone_number VARCHAR(20) UNIQUE NOT NULL,
			solana_pubkey VARCHAR(44) NOT NULL,
			encrypted_priv_key TEXT NOT NULL,
			name VARCHAR(100),
			state_version INTEGER DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`

	// Create transfers table
	transfersSQL := `
		CREATE TABLE IF NOT EXISTS transfers (
			id SERIAL PRIMARY KEY,
			sender_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			recipient_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			amount_usdt DECIMAL(20,6) NOT NULL,
			fees_usdt DECIMAL(20,6) NOT NULL,
			status VARCHAR(20) DEFAULT 'pending',
			solana_tx_hash VARCHAR(88) UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL,
			claimed_at TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_transfers_sender ON transfers(sender_id);
		CREATE INDEX IF NOT EXISTS idx_transfers_recipient ON transfers(recipient_id);
		CREATE INDEX IF NOT EXISTS idx_transfers_status ON transfers(status);
	`

	// Create withdrawals table
	withdrawalsSQL := `
		CREATE TABLE IF NOT EXISTS withdrawals (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			amount_fcfa DECIMAL(20,0) NOT NULL,
			phone_number VARCHAR(20) NOT NULL,
			provider VARCHAR(10),
			api_tx_id VARCHAR(100),
			status VARCHAR(20) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_withdrawals_user ON withdrawals(user_id);
	`

	pool := postgresDB.GetPool()

	if _, err := pool.Exec(ctx, usersSQL); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	if _, err := pool.Exec(ctx, transfersSQL); err != nil {
		return fmt.Errorf("failed to create transfers table: %w", err)
	}

	if _, err := pool.Exec(ctx, withdrawalsSQL); err != nil {
		return fmt.Errorf("failed to create withdrawals table: %w", err)
	}
	return nil
}

// SetupTestDB initializes test database
func SetupTestDB(t *testing.T) *TestDB {
	postgresDB, err := db.NewPostgresDBWithConfig(TestDBDSN, 3, 1)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	if err := postgresDB.Alive(); err != nil {
		postgresDB.Close()
		t.Fatalf("Database is not alive: %v", err)
	}

	// Initialize database schema
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := InitializeTestDatabase(ctx, postgresDB); err != nil {
		postgresDB.Close()
		t.Fatalf("Failed to initialize test database: %v", err)
	}

	return &TestDB{PostgresDB: postgresDB, T: t}
}

// SetupTestContext initializes both DB and cache for integration tests
func SetupTestContext(t *testing.T) *TestContext {
	testDB := SetupTestDB(t)

	cachePath := t.TempDir()
	testCache, err := cache.NewCache(cachePath, nil)
	if err != nil {
		defer testDB.Close()
		t.Fatalf("Failed to create cache: %v", err)
	}

	tc := &TestContext{
		DB:    testDB,
		Cache: testCache,
		T:     t,
	}

	return tc
}

// CleanupTestDB closes database connection
func (tdb *TestDB) Close() {
	if tdb.PostgresDB != nil {
		tdb.PostgresDB.Close()
	}
}

// CleanupTestContext closes all test resources
func (tc *TestContext) Close() {
	if tc.DB != nil {
		tc.DB.Close()
	}
	if tc.Cache != nil {
		tc.Cache.Close()
	}
}

// TruncateAllTables removes all data from test tables
func (tc *TestContext) TruncateAllTables() {
	tables := []string{"transfers", "withdrawals", "users"}
	for _, table := range tables {
		if err := tc.TruncateTable(table); err != nil {
			tc.T.Logf("Warning: Failed to truncate %s: %v", table, err)
		}
	}
}

// TruncateTable truncates a specific table
func (tc *TestContext) TruncateTable(table string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := tc.DB.GetPool().Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
	return err
}

// CreateTestUser creates a test user and returns it
func (tc *TestContext) CreateTestUser(phone, name string, privKey solana.PrivateKey) (*models.User, error) {
	user := &models.User{
		PhoneNumber:      phone,
		Name:             name,
		SolanaPubkey:     privKey.PublicKey().String(),
		EncryptedPrivKey: "mock_encrypted_" + privKey.PublicKey().String(),
		MockPrivKey:      privKey,
		CreatedAt:        time.Now(),
		StateVersion:     1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := tc.DB.GetPool().QueryRow(ctx, `
		INSERT INTO users (phone_number, solana_pubkey, encrypted_priv_key, name, password, created_at, state_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, user.PhoneNumber, user.SolanaPubkey, user.EncryptedPrivKey, user.Name, "hashed_password", user.CreatedAt, user.StateVersion).Scan(&user.ID)

	return user, err
}

// CreateTestTransfer creates a test transfer and returns it
func (tc *TestContext) CreateTestTransfer(senderID, recipientID uint, amount, fees float64, status string, txHash string) (*models.Transfer, error) {
	transfer := &models.Transfer{
		SenderID:     senderID,
		RecipientID:  recipientID,
		AmountUSDT:   amount,
		FeesUSDT:     fees,
		Status:       status,
		SolanaTxHash: txHash,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := tc.DB.GetPool().QueryRow(ctx, `
		INSERT INTO transfers (sender_id, recipient_id, amount_usdt, fees_usdt, status, solana_tx_hash, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, transfer.SenderID, transfer.RecipientID, transfer.AmountUSDT, transfer.FeesUSDT, transfer.Status, transfer.SolanaTxHash, transfer.CreatedAt, transfer.ExpiresAt).Scan(&transfer.ID)

	return transfer, err
}

// ContextWithTimeout returns a context with timeout
func ContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}
