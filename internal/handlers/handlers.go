// Package handlers contains the HTTP handler functions for the DiasporaConnect API.
package handlers

import (
	"Diaspora/internal/middleware"
	"Diaspora/internal/models"
	"Diaspora/internal/repository"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"golang.org/x/crypto/bcrypt"
)

// RegisterRequest is the JSON body for POST /api/register.
type RegisterRequest struct {
	PhoneNumber string `json:"phone_number"`
	Name        string `json:"name"`
	Password    string `json:"password"`
}

// Register creates a new user account, hashes the password, generates a
// Solana keypair, and stores everything in the database.
//
//	POST /api/register
//	Content-Type: application/json
func Register(userRepo *repository.UserRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			middleware.JSONError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if req.PhoneNumber == "" || req.Name == "" {
			middleware.JSONError(w, http.StatusBadRequest, "phone_number and name are required")
			return
		}
		if req.Password == "" {
			middleware.JSONError(w, http.StatusBadRequest, "password is required")
			return
		}

		hashed, err := hashPassword(req.Password)
		if err != nil {
			middleware.JSONError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}

		user := &models.User{
			PhoneNumber: req.PhoneNumber,
			Name:        req.Name,
		}
		if err := userRepo.CreateUser(r.Context(), user, hashed); err != nil {
			middleware.JSONError(w, http.StatusConflict, "user already exists or database error")
			return
		}

		middleware.JSONResponse(w, http.StatusCreated, map[string]interface{}{
			"message":       "account created successfully",
			"user_id":       user.ID,
			"solana_pubkey": user.SolanaPubkey,
		})
	}
}

// LoginRequest is the JSON body for POST /api/login.
type LoginRequest struct {
	PhoneNumber string `json:"phone_number"`
	Password    string `json:"password"`
}

// Login authenticates a user and issues a JWT on success.
//
//	POST /api/login
//	Content-Type: application/json
func Login(userRepo *repository.UserRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			middleware.JSONError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if req.PhoneNumber == "" || req.Password == "" {
			middleware.JSONError(w, http.StatusBadRequest, "phone_number and password are required")
			return
		}

		user, err := userRepo.GetUserByPhone(r.Context(), req.PhoneNumber)
		if err != nil {
			middleware.JSONError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		storedHash, err := userRepo.RetrievePasswordHash(req.PhoneNumber)
		if err != nil {
			middleware.JSONError(w, http.StatusInternalServerError, "authentication error")
			return
		}

		if err := verifyPassword(req.Password, storedHash); err != nil {
			middleware.JSONError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		token, err := middleware.GenerateToken(uint(user.ID))
		if err != nil {
			middleware.JSONError(w, http.StatusInternalServerError, "failed to generate token")
			return
		}

		middleware.JSONResponse(w, http.StatusOK, map[string]interface{}{
			"token":   token,
			"user_id": user.ID,
		})
	}
}

// VerifyOTPRequest is the JSON body for POST /api/verify-otp.
type VerifyOTPRequest struct {
	PhoneNumber string `json:"phone_number"`
	OTP         string `json:"otp"`
}

// VerifyOTP validates the one-time password sent during registration.
//
//	POST /api/verify-otp
//	Content-Type: application/json
func VerifyOTP(userRepo *repository.UserRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req VerifyOTPRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			middleware.JSONError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if req.PhoneNumber == "" || req.OTP == "" {
			middleware.JSONError(w, http.StatusBadRequest, "phone_number and otp are required")
			return
		}

		if err := userRepo.VerifyOTP(req.PhoneNumber, req.OTP); err != nil {
			middleware.JSONError(w, http.StatusUnauthorized, "invalid or expired OTP")
			return
		}

		middleware.JSONResponse(w, http.StatusOK, map[string]string{
			"message": "OTP verified successfully",
		})
	}
}

// WithdrawRequest is the JSON body for POST /api/withdraw.
type WithdrawRequest struct {
	AmountUSDT float64 `json:"amount_usdt"`
	Provider   string  `json:"provider"` // "mtn" | "moov"
}

// Withdraw debits the user's USDT balance and triggers a mobile-money payout.
//
//	POST /api/withdraw
//	Authorization: Bearer <token>
//	Content-Type: application/json
func Withdraw(userRepo *repository.UserRepo, mmClient MobileMoneyClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			middleware.JSONError(w, http.StatusUnauthorized, "missing user context")
			return
		}

		var req WithdrawRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			middleware.JSONError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.AmountUSDT <= 0 {
			middleware.JSONError(w, http.StatusBadRequest, "amount_usdt must be positive")
			return
		}

		user, err := userRepo.GetUserByID(r.Context(), userID)
		if err != nil {
			middleware.JSONError(w, http.StatusNotFound, "user not found")
			return
		}

		if err := userRepo.DebitBalance(userID, strconv.FormatFloat(req.AmountUSDT, 'f', 6, 64)); err != nil {
			middleware.JSONError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		userRepo.InvalidateUser(userID)

		provider := req.Provider
		if provider == "" {
			provider = "mtn"
		}

		txID, err := mmClient.SendMoney(user.PhoneNumber, req.AmountUSDT, provider)
		if err != nil {
			middleware.JSONError(w, http.StatusInternalServerError, "mobile money transfer failed")
			return
		}

		middleware.JSONResponse(w, http.StatusOK, map[string]interface{}{
			"message":        "withdrawal initiated",
			"mobile_tx_id":   txID,
			"amount_usdt":    req.AmountUSDT,
			"provider":       provider,
		})
	}
}

// MobileMoneyClient is the interface satisfied by the real and mock mobile
// money clients.
type MobileMoneyClient interface {
	SendMoney(phone string, amount float64, provider string) (string, error)
	CheckTransactionStatus(txID string) (string, error)
}

// ---------------------------------------------------------------------------
// Password helpers
// ---------------------------------------------------------------------------

// hashPassword returns a bcrypt hash of the SHA-256 digest of password.
// Using SHA-256 first ensures bcrypt sees a fixed-length input regardless of
// the original password length.
func hashPassword(password string) (string, error) {
	digest := sha256.Sum256([]byte(password))
	hashed, err := bcrypt.GenerateFromPassword(digest[:], bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hashed), nil
}

// verifyPassword compares a plaintext password against the stored hex-encoded
// bcrypt hash.
func verifyPassword(password, storedHex string) error {
	hashed, err := hex.DecodeString(storedHex)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(password))
	return bcrypt.CompareHashAndPassword(hashed, digest[:])
}
