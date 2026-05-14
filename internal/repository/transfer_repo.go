package repository

import (
	"Diaspora/internal/cache"
	"Diaspora/internal/db"
	"Diaspora/internal/models"
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type TransferRepo struct {
	cache *cache.CacheStore
	db    *db.PostgresDB
}

func NewTransferRepo(cache *cache.CacheStore, db *db.PostgresDB) *TransferRepo {
	return &TransferRepo{cache: cache, db: db}
}

func (r *TransferRepo) CreateTransfer(ctx context.Context, tx *models.Transfer) error {
	dbTx, err := r.db.BeginTx(ctx)
	if err != nil {
		return err
	}

	if err := InsertTransfer(ctx, dbTx, tx); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			dbTx.Rollback(ctx)
		} else {
			dbTx.Commit(ctx)
		}
	}()
	return nil
}

func (r *TransferRepo) GetPendingTransfersForRecipient(recipientID uint) ([]models.Transfer, error) {
	var transfers []models.Transfer
	err := r.db.GetPool().QueryRow(context.Background(), `
		SELECT id, sender_id, recipient_id, amount_usdt, fees_usdt, status, solana_tx_hash, created_at, expires_at, claimed_at
		FROM transfers
		WHERE recipient_id = $1 AND status = $2
		ORDER BY created_at DESC
	`, recipientID, "pending").Scan(&transfers)
	return transfers, err
}

func (r *TransferRepo) GetTransferByHash(hash string) (*models.Transfer, error) {
	var tx models.Transfer
	err := r.db.GetPool().QueryRow(context.Background(), `
		SELECT id, sender_id, recipient_id, amount_usdt, fees_usdt, status, solana_tx_hash, created_at, expires_at, claimed_at
		FROM transfers
		WHERE solana_tx_hash = $1
	`, hash).Scan(&tx)
	return &tx, err
}

func (r *TransferRepo) UpdateTransferStatus(id uint, status string, claimedAt *time.Time) error {
	updates := map[string]any{"status": status}
	if claimedAt != nil {
		updates["claimed_at"] = claimedAt
	}
	return r.db.GetPool().QueryRow(context.Background(), `
		UPDATE transfers
		SET status = $1, claimed_at = $2
		WHERE id = $3
	`, updates["status"], updates["claimed_at"], id).Scan(&id)
}

// InvalidateTransferCaches – invalide les caches de l'expéditeur et du destinataire
func (r *TransferRepo) InvalidateTransferCaches(senderID, recipientID uint, userRepo *UserRepo) error {
	_ = userRepo.InvalidateUser(senderID)
	_ = userRepo.InvalidateUser(recipientID)
	return nil
}

// InsertTransfer inserts a transfer inside an existing transaction
func InsertTransfer(ctx context.Context, tx pgx.Tx, t *models.Transfer) error {
	query := `
        INSERT INTO transfers (
            id, sender_id, recipient_id, amount_usdt, fees_usdt, status, solana_tx_hash, expires_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id
    `

	err := tx.QueryRow(
		ctx,
		query,
		t.ID,
		t.SenderID,
		t.RecipientID,
		t.AmountUSDT,
		t.FeesUSDT,
		t.Status,
		t.SolanaTxHash,
		t.ExpiresAt,
	).Scan(&t.ID)

	if err != nil {
		return fmt.Errorf("insert transfer failed: %w", err)
	}
	return nil
}
