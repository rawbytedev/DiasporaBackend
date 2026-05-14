package db_test

import (
	"Diaspora/internal/db"
	"Diaspora/internal/testhelpers"
	"context"
	"fmt"
	"testing"
	"time"
)

func TestDatabaseConnection(t *testing.T) {
	tests := []struct {
		name   string
		testFn func(*testing.T)
	}{
		{
			name: "Alive - successful connection",
			testFn: func(t *testing.T) {
				testDB := testhelpers.SetupTestDB(t)
				defer testDB.Close()
				if err := testDB.Alive(); err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			},
		},
		{
			name: "Alive - returns error on bad connection",
			testFn: func(t *testing.T) {
				badDSN := "host=localhost port=9999 user=InvalidDias password=Invalid dbname=invalid sslmode=disable"
				badPost, err := db.NewPostgresDB(badDSN)
				if err != nil {
					t.Fatal(err)
				}
				err = badPost.Alive()
				if err == nil {
					t.Error("Expected connection error, got nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFn(t)
		})
	}
}

func TestDatabaseTransactions(t *testing.T) {
	testDB := testhelpers.SetupTestDB(t)
	defer testDB.Close()

	tests := []struct {
		name      string
		operation func(context.Context, *db.PostgresDB) error
		wantErr   bool
	}{
		{
			name: "BeginTx - starts transaction successfully",
			operation: func(ctx context.Context, testdb *db.PostgresDB) error {
				tx, err := testdb.BeginTx(ctx)
				if err != nil {
					return err
				}
				defer tx.Rollback(ctx)
				return nil
			},
			wantErr: false,
		},
		{
			name: "Transaction - commit succeeds",
			operation: func(ctx context.Context, testdb *db.PostgresDB) error {
				tx, err := testdb.BeginTx(ctx)
				if err != nil {
					return err
				}

				if err := tx.Commit(ctx); err != nil {
					return err
				}
				return nil
			},
			wantErr: false,
		},
		{
			name: "Transaction - query and scan within transaction",
			operation: func(ctx context.Context, testdb *db.PostgresDB) error {
				tx, err := testdb.BeginTx(ctx)
				if err != nil {
					return err
				}
				defer tx.Rollback(ctx)

				var val int
				err = tx.QueryRow(ctx, "SELECT 42").Scan(&val)
				if err != nil {
					return err
				}
				if val != 42 {
					return fmt.Errorf("expected 42, got %d", val)
				}
				return nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := testhelpers.ContextWithTimeout(5 * time.Second)
			defer cancel()

			err := tt.operation(ctx, testDB.PostgresDB)
			if (err != nil) != tt.wantErr {
				t.Errorf("operation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDatabaseOperations(t *testing.T) {
	testCtx := testhelpers.SetupTestContext(t)
	defer testCtx.Close()

	defer testCtx.TruncateAllTables()

	tests := []struct {
		name      string
		operation func(*testing.T, *testhelpers.TestContext) error
	}{
		{
			name: "Insert user and retrieve",
			operation: func(t *testing.T, tc *testhelpers.TestContext) error {
				ctx, cancel := testhelpers.ContextWithTimeout(5 * time.Second)
				defer cancel()

				// Insert a user
				userID := 0
				err := tc.DB.GetPool().QueryRow(ctx, `
					INSERT INTO users (phone_number, solana_pubkey, encrypted_priv_key, name, password, state_version, created_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7)
					RETURNING id
				`, "+229123456", "test_solana_key", "encrypted_key", "Test User", "hashed_pass", 1, time.Now()).Scan(&userID)

				if err != nil {
					return fmt.Errorf("insert failed: %w", err)
				}

				if userID == 0 {
					return fmt.Errorf("expected non-zero user ID")
				}

				// Retrieve the user
				var phone string
				err = tc.DB.GetPool().QueryRow(ctx, "SELECT phone_number FROM users WHERE id = $1", userID).Scan(&phone)
				if err != nil {
					return fmt.Errorf("retrieve failed: %w", err)
				}

				if phone != "+229123456" {
					return fmt.Errorf("expected +229123456, got %s", phone)
				}
				return nil
			},
		},
		{
			name: "Update user state version",
			operation: func(t *testing.T, tc *testhelpers.TestContext) error {
				ctx, cancel := testhelpers.ContextWithTimeout(5 * time.Second)
				defer cancel()

				// Insert a user
				userID := 0
				err := tc.DB.GetPool().QueryRow(ctx, `
					INSERT INTO users (phone_number, solana_pubkey, encrypted_priv_key, name, password, state_version, created_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7)
					RETURNING id
				`, "+229654321", "test_key", "enc_key", "Test User", "pass", 1, time.Now()).Scan(&userID)

				if err != nil {
					return fmt.Errorf("insert failed: %w", err)
				}

				// Update state_version
				_, err = tc.DB.GetPool().Exec(ctx, "UPDATE users SET state_version = state_version + 1 WHERE id = $1", userID)
				if err != nil {
					return fmt.Errorf("update failed: %w", err)
				}

				// Verify update
				var version int
				err = tc.DB.GetPool().QueryRow(ctx, "SELECT state_version FROM users WHERE id = $1", userID).Scan(&version)
				if err != nil {
					return fmt.Errorf("verify failed: %w", err)
				}

				if version != 2 {
					return fmt.Errorf("expected version 2, got %d", version)
				}
				return nil
			},
		},
		{
			name: "Transfer insertion with foreign keys",
			operation: func(t *testing.T, tc *testhelpers.TestContext) error {
				ctx, cancel := testhelpers.ContextWithTimeout(5 * time.Second)
				defer cancel()

				// Create sender and recipient users
				senderID, recipientID := 0, 0
				err := tc.DB.GetPool().QueryRow(ctx, `
					INSERT INTO users (phone_number, solana_pubkey, encrypted_priv_key, name, password, created_at)
					VALUES ($1, $2, $3, $4, $5, $6)
					RETURNING id
				`, "+229001", "sender_key", "enc", "Sender", "pass", time.Now()).Scan(&senderID)
				if err != nil {
					return fmt.Errorf("sender insert failed: %w", err)
				}

				err = tc.DB.GetPool().QueryRow(ctx, `
					INSERT INTO users (phone_number, solana_pubkey, encrypted_priv_key, name, password, created_at)
					VALUES ($1, $2, $3, $4, $5, $6)
					RETURNING id
				`, "+229002", "recipient_key", "enc", "Recipient", "pass", time.Now()).Scan(&recipientID)
				if err != nil {
					return fmt.Errorf("recipient insert failed: %w", err)
				}

				// Create transfer
				transferID := 0
				expiresAt := time.Now().Add(7 * 24 * time.Hour)
				err = tc.DB.GetPool().QueryRow(ctx, `
					INSERT INTO transfers (sender_id, recipient_id, amount_usdt, fees_usdt, status, solana_tx_hash, created_at, expires_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
					RETURNING id
				`, senderID, recipientID, 100.0, 1.0, "pending", "tx_hash_123", time.Now(), expiresAt).Scan(&transferID)

				if err != nil {
					return fmt.Errorf("transfer insert failed: %w", err)
				}

				if transferID == 0 {
					return fmt.Errorf("expected non-zero transfer ID")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCtx.TruncateAllTables()
			if err := tt.operation(t, testCtx); err != nil {
				t.Errorf("operation() error = %v", err)
			}
		})
	}
}
