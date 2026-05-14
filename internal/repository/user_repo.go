package repository

import (
	"Diaspora/internal/cache"
	"Diaspora/internal/db"
	"Diaspora/internal/models"
	"Diaspora/internal/solana"
	"Diaspora/internal/utils"
	"context"
	"encoding/base64"
	"fmt"
	"time"
)

type UserRepo struct {
	cache  *cache.CacheStore
	db     *db.PostgresDB
	client *solana.Client
}

func (r *UserRepo) GetUserByID(context context.Context, userID uint) (*models.User, error) {
	var user models.User
	err := r.cache.Get(context, fmt.Sprintf("user:id:%d", userID), &user)
	if err == nil {
		return &user, nil
	}
	dbTx, err := r.db.BeginTx(context)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			dbTx.Rollback(context)
		} else {
			dbTx.Commit(context)
		}
	}()
	dbTx.QueryRow(context, "SELECT id, phone_number, solana_pubkey, encrypted_priv_key, name, state_version, created_at FROM users WHERE id = $1", userID).Scan(&user.ID, &user.PhoneNumber, &user.SolanaPubkey, &user.EncryptedPrivKey, &user.Name, &user.StateVersion, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) DebitBalance(userID uint, amount string) error {
	var user models.User
	err := r.db.GetPool().QueryRow(context.Background(), "SELECT id, phone_number, solana_pubkey, encrypted_priv_key, name, state_version, created_at FROM users WHERE id = $1", userID).Scan(&user.ID, &user.PhoneNumber, &user.SolanaPubkey, &user.EncryptedPrivKey, &user.Name, &user.StateVersion, &user.CreatedAt)
	if err != nil {
		return err
	}
	balance, err := r.client.GetTokenBalance(user.SolanaPubkey)
	if err != nil {
		return err
	}
	if balance < utils.ParseAmount(amount) {
		return fmt.Errorf("insufficient balance")
	}
	// implémenter la débit du solde
	return nil
}

func NewUserRepo(cache *cache.CacheStore, db *db.PostgresDB) *UserRepo {
	return &UserRepo{cache: cache, db: db /* client: solana.NewClient()*/}
}

// CreateUser – stocke en DB, pas de cache direct
func (r *UserRepo) CreateUser(ctx context.Context, user *models.User, passw string) error {
	solanaPubkey, encryptedPrivKey, err := utils.NewSolanaAddress()
	if err != nil {
		return err
	}
	user.SolanaPubkey = solanaPubkey
	user.EncryptedPrivKey = base64.StdEncoding.EncodeToString(encryptedPrivKey) // store as string (or hex)
	user.CreatedAt = time.Now()
	user.StateVersion = 1

	dbTx, err := r.db.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			dbTx.Rollback(ctx)
		} else {
			dbTx.Commit(ctx)
		}
	}()

	err = dbTx.QueryRow(ctx, `
        INSERT INTO users (phone_number, solana_pubkey, encrypted_priv_key, name, password)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id`,
		user.PhoneNumber, user.SolanaPubkey, user.EncryptedPrivKey, user.Name, passw,
	).Scan(&user.ID)

	return err
}

// GetUserByPhone – avec cache
func (r *UserRepo) GetUserByPhone(ctx context.Context, phone string) (*models.User, error) {
	key := fmt.Sprintf("user:phone:%s", phone)
	var user models.User
	err := r.cache.Get(ctx, key, &user)
	if err == nil {
		return &user, nil
	}
	dbTx, err := r.db.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			dbTx.Rollback(ctx)
		} else {
			dbTx.Commit(ctx)
		}
	}()
	// cache miss
	err = dbTx.QueryRow(ctx, "SELECT id, phone_number, solana_pubkey, encrypted_priv_key, name, state_version, created_at FROM users WHERE phone_number = $1", phone).Scan(&user.ID, &user.PhoneNumber, &user.SolanaPubkey, &user.EncryptedPrivKey, &user.Name, &user.StateVersion, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	// stocker en cache pour 5 minutes
	_ = r.cache.Set(ctx, key, user) // with expiration if supported 5*time.Minute
	return &user, nil
}

// InvalidateUser – appelé après modification (envoi, réclamation, etc.)
func (r *UserRepo) InvalidateUser(userID uint) error {
	prefix := fmt.Sprintf("user:%d:", userID)
	return r.cache.InvalidatePrefix(context.Background(), prefix)
}

// GetUserBalance – lit depuis Solana (ou cache balance)
func (r *UserRepo) GetUserBalance(userID uint, solanaClient *solana.Client) (float64, error) {
	cacheKey := fmt.Sprintf("user:%d:balance", userID)
	var balance float64
	if err := r.cache.Get(context.Background(), cacheKey, &balance); err == nil {
		return balance, nil
	}
	// récupérer l'adresse Solana
	var user models.User
	if err := r.db.GetPool().QueryRow(context.TODO(), "SELECT solana_pubkey FROM users WHERE id = $1", userID).Scan(&user.SolanaPubkey); err != nil {
		return 0, err
	}
	balance, err := solanaClient.GetTokenBalance(user.SolanaPubkey)
	if err != nil {
		return 0, err
	}
	_ = r.cache.Set(context.Background(), cacheKey, balance) // with expiration if supported 30*time.Minute
	return balance, nil
}

// UpdateStateVersion – incrémente pour invalider tous les caches de l'utilisateur
func (r *UserRepo) UpdateStateVersion(userID uint) error {
	_, err := r.db.GetPool().Exec(context.Background(), "UPDATE users SET state_version = state_version + 1 WHERE id = $1", userID)
	if err != nil {
		return err
	}
	return nil
}

func (r *UserRepo) RetrievePasswordHash(phone string) (string, error) {
	var passwordHash string
	err := r.db.GetPool().QueryRow(context.Background(), "SELECT password FROM users WHERE phone_number = $1", phone).Scan(&passwordHash)
	if err != nil {
		return "", err
	}
	return passwordHash, nil
}

// opt string to be removed, OTP generation should be internal to the repo and not passed in from outside
func (r *UserRepo) StoreOTP(phone, otp string) error {
	// pour l'exemple, on accepte "123456" comme OTP valide
	if otp == "" {
		otp = utils.GenerateOTP()
	}
	return r.cache.Set(context.Background(), fmt.Sprintf("otp:%s", phone), otp) // stocker OTP pour vérification
}

func (r *UserRepo) VerifyOTP(phone, otp string) error {
	var expectedOTP string
	err := r.cache.Get(context.Background(), fmt.Sprintf("otp:%s", phone), &expectedOTP)
	if err != nil {
		return fmt.Errorf("OTP not found")
	}
	if otp != expectedOTP {
		return fmt.Errorf("invalid OTP")
	}
	return nil
}

func (r *UserRepo) Close() error {
	return r.cache.Close()
}
