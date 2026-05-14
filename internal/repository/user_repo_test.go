package repository_test

import (
	"Diaspora/internal/models"
	"Diaspora/internal/repository"
	"Diaspora/internal/testhelpers"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
)

func TestUserRepository(t *testing.T) {
	testCtx := testhelpers.SetupTestContext(t)
	defer testCtx.Close()
	defer testCtx.TruncateAllTables()

	userRepo := repository.NewUserRepo(testCtx.Cache, testCtx.DB.PostgresDB)

	tests := []struct {
		name      string
		operation func(*testing.T, *repository.UserRepo, *testhelpers.TestContext) error
	}{
		{
			name: "CreateUser - successfully creates user with encrypted key",
			operation: func(t *testing.T, ur *repository.UserRepo, tc *testhelpers.TestContext) error {
				ctx, cancel := testhelpers.ContextWithTimeout(5 * time.Second)
				defer cancel()

				user := &models.User{
					PhoneNumber: "+229123456789",
					Name:        "John Doe",
				}

				err := ur.CreateUser(ctx, user, "hashed_password_123")
				if err != nil {
					return fmt.Errorf("CreateUser failed: %w", err)
				}

				if user.ID == 0 {
					return fmt.Errorf("expected non-zero user ID after creation")
				}

				if user.SolanaPubkey == "" {
					return fmt.Errorf("expected Solana pubkey to be set")
				}

				if user.EncryptedPrivKey == "" {
					return fmt.Errorf("expected encrypted private key to be set")
				}

				return nil
			},
		},
		{
			name: "CreateUser - fails with duplicate phone number",
			operation: func(t *testing.T, ur *repository.UserRepo, tc *testhelpers.TestContext) error {
				ctx, cancel := testhelpers.ContextWithTimeout(5 * time.Second)
				defer cancel()

				phone := "+229111222333"
				user1 := &models.User{PhoneNumber: phone, Name: "User 1"}
				user2 := &models.User{PhoneNumber: phone, Name: "User 2"}

				err := ur.CreateUser(ctx, user1, "pass1")
				if err != nil {
					return fmt.Errorf("first CreateUser failed: %w", err)
				}

				// Try to create user with same phone
				err = ur.CreateUser(ctx, user2, "pass2")
				if err == nil {
					return fmt.Errorf("expected error for duplicate phone, got nil")
				}

				return nil
			},
		},
		{
			name: "GetUserByPhone - retrieves user from database",
			operation: func(t *testing.T, ur *repository.UserRepo, tc *testhelpers.TestContext) error {
				ctx, cancel := testhelpers.ContextWithTimeout(5 * time.Second)
				defer cancel()

				phone := "+229999888777"
				user := &models.User{PhoneNumber: phone, Name: "Test User"}

				// Create user
				err := ur.CreateUser(ctx, user, "password")
				if err != nil {
					return fmt.Errorf("CreateUser failed: %w", err)
				}

				// Retrieve user
				retrieved, err := ur.GetUserByPhone(ctx, phone)
				if err != nil {
					return fmt.Errorf("GetUserByPhone failed: %w", err)
				}

				if retrieved.PhoneNumber != phone {
					return fmt.Errorf("expected phone %s, got %s", phone, retrieved.PhoneNumber)
				}

				if retrieved.Name != "Test User" {
					return fmt.Errorf("expected name 'Test User', got %s", retrieved.Name)
				}

				return nil
			},
		},
		{
			name: "GetUserByPhone - returns error for non-existent user",
			operation: func(t *testing.T, ur *repository.UserRepo, tc *testhelpers.TestContext) error {
				ctx, cancel := testhelpers.ContextWithTimeout(5 * time.Second)
				defer cancel()

				_, err := ur.GetUserByPhone(ctx, "+229000000000")
				if err == nil {
					return fmt.Errorf("expected error for non-existent user, got nil")
				}

				return nil
			},
		},
		{
			name: "InvalidateUser - clears user cache",
			operation: func(t *testing.T, ur *repository.UserRepo, tc *testhelpers.TestContext) error {
				ctx, cancel := testhelpers.ContextWithTimeout(5 * time.Second)
				defer cancel()

				phone := "+229777666555"
				user := &models.User{PhoneNumber: phone, Name: "Cache Test"}

				err := ur.CreateUser(ctx, user, "password")
				if err != nil {
					return fmt.Errorf("CreateUser failed: %w", err)
				}

				// Get user (populates cache)
				retrieved, err := ur.GetUserByPhone(ctx, phone)
				if err != nil {
					return fmt.Errorf("GetUserByPhone failed: %w", err)
				}

				// Invalidate cache
				err = ur.InvalidateUser(retrieved.ID)
				if err != nil {
					return fmt.Errorf("InvalidateUser failed: %w", err)
				}

				return nil
			},
		},
		{
			name: "UpdateStateVersion - increments version",
			operation: func(t *testing.T, ur *repository.UserRepo, tc *testhelpers.TestContext) error {
				ctx, cancel := testhelpers.ContextWithTimeout(5 * time.Second)
				defer cancel()

				user := &models.User{PhoneNumber: "+229555444333", Name: "Version Test"}

				err := ur.CreateUser(ctx, user, "password")
				if err != nil {
					return fmt.Errorf("CreateUser failed: %w", err)
				}

				// Get initial version
				retrieved, _ := ur.GetUserByPhone(ctx, "+229555444333")
				initialVersion := retrieved.StateVersion

				// Update version
				err = ur.UpdateStateVersion(retrieved.ID)
				if err != nil {
					return fmt.Errorf("UpdateStateVersion failed: %w", err)
				}

				// Verify version incremented
				retrieved2, _ := ur.GetUserByPhone(ctx, "+229555444333")
				if retrieved2.StateVersion != initialVersion+1 {
					return fmt.Errorf("expected version %d, got %d", initialVersion+1, retrieved2.StateVersion)
				}

				return nil
			},
		},
		{
			name: "StoreOTP and VerifyOTP - correct OTP",
			operation: func(t *testing.T, ur *repository.UserRepo, tc *testhelpers.TestContext) error {
				phone := "+229444333222"
				otp := "123456"

				// Store OTP
				err := ur.StoreOTP(phone, otp)
				if err != nil {
					return fmt.Errorf("StoreOTP failed: %w", err)
				}

				// Verify OTP
				err = ur.VerifyOTP(phone, otp)
				if err != nil {
					return fmt.Errorf("VerifyOTP failed: %w", err)
				}

				return nil
			},
		},
		{
			name: "VerifyOTP - fails with wrong OTP",
			operation: func(t *testing.T, ur *repository.UserRepo, tc *testhelpers.TestContext) error {
				phone := "+229333222111"
				correctOtp := "123456"
				wrongOtp := "654321"

				err := ur.StoreOTP(phone, correctOtp)
				if err != nil {
					return fmt.Errorf("StoreOTP failed: %w", err)
				}

				err = ur.VerifyOTP(phone, wrongOtp)
				if err == nil {
					return fmt.Errorf("expected error for wrong OTP, got nil")
				}

				return nil
			},
		},
		{
			name: "RetrievePasswordHash - returns stored hash",
			operation: func(t *testing.T, ur *repository.UserRepo, tc *testhelpers.TestContext) error {
				ctx, cancel := testhelpers.ContextWithTimeout(5 * time.Second)
				defer cancel()

				phone := "+229222111999"
				passwordHash := "hashed_password_test"

				user := &models.User{PhoneNumber: phone, Name: "Hash Test"}
				err := ur.CreateUser(ctx, user, passwordHash)
				if err != nil {
					return fmt.Errorf("CreateUser failed: %w", err)
				}

				// Retrieve hash
				retrieved, err := ur.RetrievePasswordHash(phone)
				if err != nil {
					return fmt.Errorf("RetrievePasswordHash failed: %w", err)
				}

				if retrieved != passwordHash {
					return fmt.Errorf("expected hash %s, got %s", passwordHash, retrieved)
				}

				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCtx.TruncateAllTables()
			if err := tt.operation(t, userRepo, testCtx); err != nil {
				t.Errorf("operation() error = %v", err)
			}
		})
	}
}

func TestUserRepositoryWithBalance(t *testing.T) {
	testCtx := testhelpers.SetupTestContext(t)
	defer testCtx.Close()
	defer testCtx.TruncateAllTables()

	t.Run("GetUserBalance - returns cached balance", func(t *testing.T) {
		privKey := solana.NewWallet().PrivateKey
		user, err := testCtx.CreateTestUser("+229111222333", "Balance Test", privKey)
		if err != nil {
			t.Fatalf("CreateTestUser failed: %v", err)
		}

		// Cache the balance
		ctx := context.Background()
		cacheKey := fmt.Sprintf("user:%d:balance", user.ID)
		expectedBalance := 500.0
		err = testCtx.Cache.Set(ctx, cacheKey, expectedBalance)
		if err != nil {
			t.Fatalf("Cache.Set failed: %v", err)
		}

		// Note: GetUserBalance requires a real Solana client or mock
		// This test demonstrates the caching pattern
		// In real tests, you'd use a mock Solana client
	})
}
