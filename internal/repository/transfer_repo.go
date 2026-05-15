// Package repository contains the data-access layer for the DiasporaConnect API.
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

// TransferRepo manages Transfer persistence in PostgreSQL with a BadgerDB
// read-through cache for hot paths.
type TransferRepo struct {
        cache *cache.CacheStore
        db    *db.PostgresDB
}

// NewTransferRepo creates a new TransferRepo.
func NewTransferRepo(cache *cache.CacheStore, database *db.PostgresDB) *TransferRepo {
        return &TransferRepo{cache: cache, db: database}
}

// CreateTransfer inserts a new transfer record inside a database transaction.
func (r *TransferRepo) CreateTransfer(ctx context.Context, tx *models.Transfer) (err error) {
        dbTx, err := r.db.BeginTx(ctx)
        if err != nil {
                return err
        }
        defer func() {
                if err != nil {
                        _ = dbTx.Rollback(ctx)
                } else {
                        _ = dbTx.Commit(ctx)
                }
        }()
        return InsertTransfer(ctx, dbTx, tx)
}

// GetTransferByID fetches a single transfer by its primary key.
func (r *TransferRepo) GetTransferByID(ctx context.Context, id uint) (*models.Transfer, error) {
        var tx models.Transfer
        err := r.db.GetPool().QueryRow(ctx, `
                SELECT id, sender_id, recipient_id, amount_usdt, fees_usdt, status,
                       solana_tx_hash, created_at, expires_at, claimed_at
                FROM transfers
                WHERE id = $1
        `, id).Scan(
                &tx.ID, &tx.SenderID, &tx.RecipientID,
                &tx.AmountUSDT, &tx.FeesUSDT, &tx.Status,
                &tx.SolanaTxHash, &tx.CreatedAt, &tx.ExpiresAt, &tx.ClaimedAt,
        )
        if err != nil {
                return nil, fmt.Errorf("GetTransferByID %d: %w", id, err)
        }
        return &tx, nil
}

// GetTransferByHash fetches a single transfer by its Solana transaction signature.
func (r *TransferRepo) GetTransferByHash(ctx context.Context, hash string) (*models.Transfer, error) {
        var tx models.Transfer
        err := r.db.GetPool().QueryRow(ctx, `
                SELECT id, sender_id, recipient_id, amount_usdt, fees_usdt, status,
                       solana_tx_hash, created_at, expires_at, claimed_at
                FROM transfers
                WHERE solana_tx_hash = $1
        `, hash).Scan(
                &tx.ID, &tx.SenderID, &tx.RecipientID,
                &tx.AmountUSDT, &tx.FeesUSDT, &tx.Status,
                &tx.SolanaTxHash, &tx.CreatedAt, &tx.ExpiresAt, &tx.ClaimedAt,
        )
        if err != nil {
                return nil, fmt.Errorf("GetTransferByHash %q: %w", hash, err)
        }
        return &tx, nil
}

// GetPendingTransfersForRecipient returns transfers the user can still claim.
func (r *TransferRepo) GetPendingTransfersForRecipient(ctx context.Context, recipientID uint) ([]models.Transfer, error) {
        return r.queryTransfers(ctx, `
                SELECT id, sender_id, recipient_id, amount_usdt, fees_usdt, status,
                       solana_tx_hash, created_at, expires_at, claimed_at
                FROM transfers
                WHERE recipient_id = $1 AND status = 'pending'
                ORDER BY created_at DESC
        `, recipientID)
}

// GetTransfersByUserID returns all transfers where the user is either sender or
// recipient.  The optional status filter (e.g. "pending", "claimed") narrows
// the results; pass "" to return all statuses.
func (r *TransferRepo) GetTransfersByUserID(ctx context.Context, userID uint, status string) ([]models.Transfer, error) {
        if status != "" {
                return r.queryTransfers(ctx, `
                        SELECT id, sender_id, recipient_id, amount_usdt, fees_usdt, status,
                               solana_tx_hash, created_at, expires_at, claimed_at
                        FROM transfers
                        WHERE (sender_id = $1 OR recipient_id = $1) AND status = $2
                        ORDER BY created_at DESC
                `, userID, status)
        }
        return r.queryTransfers(ctx, `
                SELECT id, sender_id, recipient_id, amount_usdt, fees_usdt, status,
                       solana_tx_hash, created_at, expires_at, claimed_at
                FROM transfers
                WHERE sender_id = $1 OR recipient_id = $1
                ORDER BY created_at DESC
        `, userID)
}

// GetSentTransfers returns transfers initiated by the user, optionally filtered
// by status.
func (r *TransferRepo) GetSentTransfers(ctx context.Context, userID uint, status string) ([]models.Transfer, error) {
        if status != "" {
                return r.queryTransfers(ctx, `
                        SELECT id, sender_id, recipient_id, amount_usdt, fees_usdt, status,
                               solana_tx_hash, created_at, expires_at, claimed_at
                        FROM transfers
                        WHERE sender_id = $1 AND status = $2
                        ORDER BY created_at DESC
                `, userID, status)
        }
        return r.queryTransfers(ctx, `
                SELECT id, sender_id, recipient_id, amount_usdt, fees_usdt, status,
                       solana_tx_hash, created_at, expires_at, claimed_at
                FROM transfers
                WHERE sender_id = $1
                ORDER BY created_at DESC
        `, userID)
}

// GetReceivedTransfers returns transfers where the user is the recipient,
// optionally filtered by status.
func (r *TransferRepo) GetReceivedTransfers(ctx context.Context, userID uint, status string) ([]models.Transfer, error) {
        if status != "" {
                return r.queryTransfers(ctx, `
                        SELECT id, sender_id, recipient_id, amount_usdt, fees_usdt, status,
                               solana_tx_hash, created_at, expires_at, claimed_at
                        FROM transfers
                        WHERE recipient_id = $1 AND status = $2
                        ORDER BY created_at DESC
                `, userID, status)
        }
        return r.queryTransfers(ctx, `
                SELECT id, sender_id, recipient_id, amount_usdt, fees_usdt, status,
                       solana_tx_hash, created_at, expires_at, claimed_at
                FROM transfers
                WHERE recipient_id = $1
                ORDER BY created_at DESC
        `, userID)
}

// UpdateTransferStatus changes a transfer's status and, for "claimed",
// records the timestamp.
func (r *TransferRepo) UpdateTransferStatus(ctx context.Context, id uint, status string, claimedAt *time.Time) error {
        _, err := r.db.GetPool().Exec(ctx, `
                UPDATE transfers
                SET status = $1, claimed_at = $2
                WHERE id = $3
        `, status, claimedAt, id)
        return err
}

// InvalidateTransferCaches clears user-level cache entries for both parties.
func (r *TransferRepo) InvalidateTransferCaches(senderID, recipientID uint, userRepo *UserRepo) error {
        _ = userRepo.InvalidateUser(senderID)
        _ = userRepo.InvalidateUser(recipientID)
        return nil
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// InsertTransfer inserts a transfer inside an existing pgx.Tx and populates
// the transfer's ID on success.
func InsertTransfer(ctx context.Context, tx pgx.Tx, t *models.Transfer) error {
        err := tx.QueryRow(ctx, `
                INSERT INTO transfers (
                        sender_id, recipient_id, amount_usdt, fees_usdt,
                        status, solana_tx_hash, escrow_nonce, expires_at
                ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
                RETURNING id
        `,
                t.SenderID, t.RecipientID, t.AmountUSDT, t.FeesUSDT,
                t.Status, t.SolanaTxHash, t.EscrowNonce, t.ExpiresAt,
        ).Scan(&t.ID)
        if err != nil {
                return fmt.Errorf("insert transfer: %w", err)
        }
        return nil
}

// queryTransfers is a generic row scanner for transfer lists.
func (r *TransferRepo) queryTransfers(ctx context.Context, query string, args ...interface{}) ([]models.Transfer, error) {
        rows, err := r.db.GetPool().Query(ctx, query, args...)
        if err != nil {
                return nil, err
        }
        defer rows.Close()

        var transfers []models.Transfer
        for rows.Next() {
                var tx models.Transfer
                if err := rows.Scan(
                        &tx.ID, &tx.SenderID, &tx.RecipientID,
                        &tx.AmountUSDT, &tx.FeesUSDT, &tx.Status,
                        &tx.SolanaTxHash, &tx.CreatedAt, &tx.ExpiresAt, &tx.ClaimedAt,
                ); err != nil {
                        return nil, err
                }
                transfers = append(transfers, tx)
        }
        return transfers, rows.Err()
}
