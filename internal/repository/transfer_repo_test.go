package repository_test

import (
        "Diaspora/internal/models"
        "Diaspora/internal/repository"
        "Diaspora/internal/testhelpers"
        "fmt"
        "testing"
        "time"

        "github.com/gagliardetto/solana-go"
)

func TestTransferRepository(t *testing.T) {
        testCtx := testhelpers.SetupTestContext(t)
        defer testCtx.Close()
        defer testCtx.TruncateAllTables()

        transferRepo := repository.NewTransferRepo(testCtx.Cache, testCtx.DB.PostgresDB)
        userRepo := repository.NewUserRepo(testCtx.Cache, testCtx.DB.PostgresDB)

        tests := []struct {
                name      string
                operation func(*testing.T, *repository.TransferRepo, *testhelpers.TestContext, uint, uint) error
        }{
                {
                        name: "CreateTransfer - successfully creates transfer record",
                        operation: func(t *testing.T, tr *repository.TransferRepo, tc *testhelpers.TestContext, senderID, recipientID uint) error {
                                transfer := &models.Transfer{
                                        SenderID:     senderID,
                                        RecipientID:  recipientID,
                                        AmountUSDT:   100.0,
                                        FeesUSDT:     1.0,
                                        Status:       "pending",
                                        SolanaTxHash: "tx_hash_001",
                                        ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
                                }

                                err := tr.CreateTransfer(tc.T.Context(), transfer)
                                if err != nil {
                                        return fmt.Errorf("CreateTransfer failed: %w", err)
                                }

                                if transfer.ID == 0 {
                                        return fmt.Errorf("expected non-zero transfer ID after creation")
                                }

                                return nil
                        },
                },
                {
                        name: "CreateTransfer - fails with duplicate tx hash",
                        operation: func(t *testing.T, tr *repository.TransferRepo, tc *testhelpers.TestContext, senderID, recipientID uint) error {
                                txHash := "tx_hash_duplicate"

                                transfer1 := &models.Transfer{
                                        SenderID:     senderID,
                                        RecipientID:  recipientID,
                                        AmountUSDT:   50.0,
                                        FeesUSDT:     0.5,
                                        Status:       "pending",
                                        SolanaTxHash: txHash,
                                        ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
                                }

                                err := tr.CreateTransfer(tc.T.Context(), transfer1)
                                if err != nil {
                                        return fmt.Errorf("first CreateTransfer failed: %w", err)
                                }

                                transfer2 := &models.Transfer{
                                        SenderID:     senderID,
                                        RecipientID:  recipientID,
                                        AmountUSDT:   75.0,
                                        FeesUSDT:     0.75,
                                        Status:       "pending",
                                        SolanaTxHash: txHash,
                                        ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
                                }

                                err = tr.CreateTransfer(tc.T.Context(), transfer2)
                                if err == nil {
                                        return fmt.Errorf("expected error for duplicate tx hash, got nil")
                                }

                                return nil
                        },
                },
                {
                        name: "GetPendingTransfersForRecipient - returns pending transfers only",
                        operation: func(t *testing.T, tr *repository.TransferRepo, tc *testhelpers.TestContext, senderID, recipientID uint) error {
                                // Create multiple transfers with different statuses
                                pending := &models.Transfer{
                                        SenderID:     senderID,
                                        RecipientID:  recipientID,
                                        AmountUSDT:   100.0,
                                        FeesUSDT:     1.0,
                                        Status:       "pending",
                                        SolanaTxHash: "tx_pending_001",
                                        ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
                                }

                                claimed := &models.Transfer{
                                        SenderID:     senderID,
                                        RecipientID:  recipientID,
                                        AmountUSDT:   200.0,
                                        FeesUSDT:     2.0,
                                        Status:       "claimed",
                                        SolanaTxHash: "tx_claimed_001",
                                        ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
                                }

                                err := tr.CreateTransfer(tc.T.Context(), pending)
                                if err != nil {
                                        return fmt.Errorf("create pending transfer failed: %w", err)
                                }

                                err = tr.CreateTransfer(tc.T.Context(), claimed)
                                if err != nil {
                                        return fmt.Errorf("create claimed transfer failed: %w", err)
                                }

                                transfers, err := tr.GetPendingTransfersForRecipient(tc.T.Context(), recipientID)
                                if err != nil {
                                        return fmt.Errorf("GetPendingTransfersForRecipient failed: %w", err)
                                }

                                if len(transfers) != 1 {
                                        return fmt.Errorf("expected 1 pending transfer, got %d", len(transfers))
                                }

                                if transfers[0].Status != "pending" {
                                        return fmt.Errorf("expected status 'pending', got %s", transfers[0].Status)
                                }

                                return nil
                        },
                },
                {
                        name: "GetTransferByHash - retrieves transfer by tx hash",
                        operation: func(t *testing.T, tr *repository.TransferRepo, tc *testhelpers.TestContext, senderID, recipientID uint) error {
                                txHash := "tx_hash_retrieve"

                                transfer := &models.Transfer{
                                        SenderID:     senderID,
                                        RecipientID:  recipientID,
                                        AmountUSDT:   150.0,
                                        FeesUSDT:     1.5,
                                        Status:       "pending",
                                        SolanaTxHash: txHash,
                                        ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
                                }

                                err := tr.CreateTransfer(tc.T.Context(), transfer)
                                if err != nil {
                                        return fmt.Errorf("CreateTransfer failed: %w", err)
                                }

                                retrieved, err := tr.GetTransferByHash(tc.T.Context(), txHash)
                                if err != nil {
                                        return fmt.Errorf("GetTransferByHash failed: %w", err)
                                }

                                if retrieved.SolanaTxHash != txHash {
                                        return fmt.Errorf("expected tx hash %s, got %s", txHash, retrieved.SolanaTxHash)
                                }

                                return nil
                        },
                },
                {
                        name: "UpdateTransferStatus - updates status to claimed with timestamp",
                        operation: func(t *testing.T, tr *repository.TransferRepo, tc *testhelpers.TestContext, senderID, recipientID uint) error {
                                transfer := &models.Transfer{
                                        SenderID:     senderID,
                                        RecipientID:  recipientID,
                                        AmountUSDT:   100.0,
                                        FeesUSDT:     1.0,
                                        Status:       "pending",
                                        SolanaTxHash: "tx_hash_update",
                                        ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
                                }

                                err := tr.CreateTransfer(tc.T.Context(), transfer)
                                if err != nil {
                                        return fmt.Errorf("CreateTransfer failed: %w", err)
                                }

                                now := time.Now()
                                err = tr.UpdateTransferStatus(tc.T.Context(), transfer.ID, "claimed", &now)
                                if err != nil {
                                        return fmt.Errorf("UpdateTransferStatus failed: %w", err)
                                }

                                // Verify status was updated
                                retrieved, _ := tr.GetTransferByHash(tc.T.Context(), "tx_hash_update")
                                if retrieved.Status != "claimed" {
                                        return fmt.Errorf("expected status 'claimed', got %s", retrieved.Status)
                                }

                                return nil
                        },
                },
                {
                        name: "InvalidateTransferCaches - clears user caches",
                        operation: func(t *testing.T, tr *repository.TransferRepo, tc *testhelpers.TestContext, senderID, recipientID uint) error {
                                // Invalidate caches for sender and recipient
                                err := tr.InvalidateTransferCaches(senderID, recipientID, userRepo)
                                if err != nil {
                                        return fmt.Errorf("InvalidateTransferCaches failed: %w", err)
                                }

                                return nil
                        },
                },
        }

        for _, tt := range tests {
                t.Run(tt.name, func(t *testing.T) {
                        testCtx.TruncateAllTables()
                        // Recreate test users for each test
                        sender := createTestUserHelper(t, testCtx, "+229111111111", "Sender")
                        recipient := createTestUserHelper(t, testCtx, "+229222222222", "Recipient")

                        if err := tt.operation(t, transferRepo, testCtx, sender.ID, recipient.ID); err != nil {
                                t.Errorf("operation() error = %v", err)
                        }
                })
        }
}

func TestTransferEdgeCases(t *testing.T) {
        testCtx := testhelpers.SetupTestContext(t)
        defer testCtx.Close()
        defer testCtx.TruncateAllTables()

        transferRepo := repository.NewTransferRepo(testCtx.Cache, testCtx.DB.PostgresDB)

        sender := createTestUserHelper(t, testCtx, "+229333333333", "Sender")
        recipient := createTestUserHelper(t, testCtx, "+229444444444", "Recipient")

        tests := []struct {
                name      string
                operation func(*testing.T) error
        }{
                {
                        name: "Transfer with zero amount",
                        operation: func(t *testing.T) error {
                                transfer := &models.Transfer{
                                        SenderID:     sender.ID,
                                        RecipientID:  recipient.ID,
                                        AmountUSDT:   0.0,
                                        FeesUSDT:     0.0,
                                        Status:       "pending",
                                        SolanaTxHash: "tx_zero_001",
                                        ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
                                }

                                err := transferRepo.CreateTransfer(testCtx.T.Context(), transfer)
                                if err != nil {
                                        return fmt.Errorf("CreateTransfer with zero amount failed: %w", err)
                                }

                                return nil
                        },
                },
                {
                        name: "Transfer with very large amount",
                        operation: func(t *testing.T) error {
                                transfer := &models.Transfer{
                                        SenderID:     sender.ID,
                                        RecipientID:  recipient.ID,
                                        AmountUSDT:   999999999.999999,
                                        FeesUSDT:     9999999.999999,
                                        Status:       "pending",
                                        SolanaTxHash: "tx_large_001",
                                        ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
                                }

                                err := transferRepo.CreateTransfer(testCtx.T.Context(), transfer)
                                if err != nil {
                                        return fmt.Errorf("CreateTransfer with large amount failed: %w", err)
                                }

                                return nil
                        },
                },
                {
                        name: "Transfer with expired date",
                        operation: func(t *testing.T) error {
                                transfer := &models.Transfer{
                                        SenderID:     sender.ID,
                                        RecipientID:  recipient.ID,
                                        AmountUSDT:   50.0,
                                        FeesUSDT:     0.5,
                                        Status:       "pending",
                                        SolanaTxHash: "tx_expired_001",
                                        ExpiresAt:    time.Now().Add(-1 * time.Hour), // Already expired
                                }

                                err := transferRepo.CreateTransfer(testCtx.T.Context(), transfer)
                                if err != nil {
                                        return fmt.Errorf("CreateTransfer with expired date failed: %w", err)
                                }

                                return nil
                        },
                },
        }

        for _, tt := range tests {
                t.Run(tt.name, func(t *testing.T) {
                        testCtx.TruncateAllTables()
                        sender = createTestUserHelper(t, testCtx, "+229333333333", "Sender")
                        recipient = createTestUserHelper(t, testCtx, "+229444444444", "Recipient")

                        if err := tt.operation(t); err != nil {
                                t.Errorf("operation() error = %v", err)
                        }
                })
        }
}

// Helper function to create test users
func createTestUserHelper(t *testing.T, tc *testhelpers.TestContext, phone, name string) *models.User {
        privKey := solana.NewWallet().PrivateKey
        user, err := tc.CreateTestUser(phone, name, privKey)
        if err != nil {
                t.Fatalf("Failed to create test user: %v", err)
        }
        return user
}
