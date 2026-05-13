package repository

import (
	"Diaspora/internal/cache"
	"Diaspora/internal/db"
	"Diaspora/internal/models"
	"Diaspora/internal/solana"
	"Diaspora/internal/utils"
	"context"
	"fmt"
	"time"
)

type UserRepo struct {
	cache *cache.CacheStore
	db    *db.PostgresDB
}

func NewUserRepo(cache *cache.CacheStore, db *db.PostgresDB) *UserRepo {
	return &UserRepo{cache: cache, db: db}
}

// CreateUser – stocke en DB, pas de cache direct
func (r *UserRepo) CreateUser(user *models.User, passw string) error {
	solanaPubkey, encryptedPrivKey, err := utils.NewSolanaAddress()
	if err != nil {
		return err
	}
	user.SolanaPubkey = solanaPubkey
	user.MockPrivKey = encryptedPrivKey
	user.EncryptedPrivKey = user.MockPrivKey.String() // In a real implementation, this would be properly encrypted with a server-side key
	user.CreatedAt = time.Now()
	user.StateVersion = 1
	user.ID = uint(time.Now().UnixNano()) // simple ID generation for example, use a better method in production
	// we avoid storing password in the User struct, it is stored separately in the database for authentication purposes
	return r.db.GetPool().QueryRow(context.Background(), "INSERT INTO users (id ,phone_number, solana_pubkey, encrypted_priv_key, name, password) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id", user.PhoneNumber, user.SolanaPubkey, user.EncryptedPrivKey, user.Name, passw).Scan(&user.ID)
}

// GetUserByPhone – avec cache
func (r *UserRepo) GetUserByPhone(phone string) (*models.User, error) {
	key := fmt.Sprintf("user:phone:%s", phone)
	var user models.User
	err := r.cache.Get(context.Background(), key, &user)
	if err == nil {
		return &user, nil
	}
	// cache miss
	err = r.db.GetPool().QueryRow(context.Background(), "SELECT id, phone_number, solana_pubkey, encrypted_priv_key, name, state_version, created_at FROM users WHERE phone_number = $1", phone).Scan(&user.ID, &user.PhoneNumber, &user.SolanaPubkey, &user.EncryptedPrivKey, &user.Name, &user.StateVersion, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	// stocker en cache pour 5 minutes
	_ = r.cache.Set(context.Background(), key, user) // with expiration if supported 5*time.Minute
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
