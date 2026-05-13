package handlers

import (
	"Diaspora/internal/mobilemoney"
	"Diaspora/internal/models"
	"Diaspora/internal/repository"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

func Register(userRepo *repository.UserRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// implémentation de l'inscription
		r.ParseForm()
		phone := r.Form.Get("phone")
		name := r.Form.Get("name")
		password := r.Form.Get("password")                                                    // for authentication
		hashed, err := bcrypt.GenerateFromPassword([]byte(password)[:72], bcrypt.DefaultCost) // bcrypt has a max password length of 72 bytes, we hash it first to ensure consistent length
		passwordHash := hex.EncodeToString(sha256.New().Sum(hashed))
		if err != nil {
			http.Error(w, "failed to hash password", http.StatusInternalServerError)
			return
		}
		if phone == "" || name == "" {
			http.Error(w, "missing phone or name", http.StatusBadRequest)
			return
		}
		// créer utilisateur en DB
		user := &models.User{
			PhoneNumber: phone,
			Name:        name,
		}
		err = userRepo.CreateUser(user, passwordHash) // stocke un hash du mot de passe pour vérification lors du login
		if err != nil {
			http.Error(w, "failed to create user", http.StatusInternalServerError)
			return
		}
	}
}

func Login(userRepo *repository.UserRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		phone := r.Form.Get("phone")
		password := r.Form.Get("password")
		if phone == "" || password == "" {
			http.Error(w, "missing phone or password", http.StatusBadRequest)
			return
		}
		_, err := userRepo.GetUserByPhone(phone)
		if err != nil {
			http.Error(w, "user not found", http.StatusUnauthorized)
			return
		}
		userPasswordHash, err := userRepo.RetrievePasswordHash(phone)
		if err != nil {
			http.Error(w, "failed to retrieve password hash", http.StatusInternalServerError)
			return
		}
		err = bcrypt.CompareHashAndPassword([]byte(userPasswordHash), []byte(password))
		if err != nil {
			http.Error(w, "invalid password", http.StatusUnauthorized)
			return
		}
		err = userRepo.StoreOTP(phone, "123456") // génère et stocke OTP, envoie par SMS
		if err != nil {
			http.Error(w, "failed to send OTP", http.StatusInternalServerError)
			return
		}
	}
}

func Withdraw(userRepo *repository.UserRepo, mobileMoneyClient *mobilemoney.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// implémentation du retrait vers mobile money
	}
}
func VerifyOTP(userRepo *repository.UserRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		phone := r.Form.Get("phone")
		otp := r.Form.Get("otp")
		if phone == "" || otp == "" {
			http.Error(w, "missing phone or otp", http.StatusBadRequest)
			return
		}
		err := userRepo.VerifyOTP(phone, otp)
		if err != nil {
			http.Error(w, "invalid otp", http.StatusUnauthorized)
			return
		}
	}
}
