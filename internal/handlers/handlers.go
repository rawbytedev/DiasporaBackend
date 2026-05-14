package handlers

import (
	"Diaspora/internal/mobilemoney"
	"Diaspora/internal/models"
	"Diaspora/internal/repository"
	"crypto/sha256"
	"encoding/hex"
	"internal/itoa"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

func Register(userRepo *repository.UserRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// implémentation de l'inscription
		r.ParseForm()
		phone := r.Form.Get("phone")
		name := r.Form.Get("name")

		password := r.Form.Get("password") // for authentication
		// validation des champs
		if phone == "" || name == "" {
			http.Error(w, "missing phone or name", http.StatusBadRequest)
			return
		}
		if password == "" {
			http.Error(w, "missing password", http.StatusBadRequest)
			return
		}
		shaHash := sha256.New().Sum([]byte(password))
		hashed, err := bcrypt.GenerateFromPassword(shaHash[:], bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "failed to hash password", http.StatusInternalServerError)
			return
		}
		hashedPassword := hex.EncodeToString(hashed)
		// créer utilisateur en DB
		user := &models.User{
			PhoneNumber: phone,
			Name:        name,
		}
		err = userRepo.CreateUser(r.Context(), user, hashedPassword) // stocke un hash du mot de passe pour vérification lors du login
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
		_, err := userRepo.GetUserByPhone(r.Context(), phone)
		if err != nil {
			http.Error(w, "user not found", http.StatusUnauthorized)
			return
		}
		userPasswordHash, err := userRepo.RetrievePasswordHash(phone)
		if err != nil {
			http.Error(w, "failed to retrieve password hash", http.StatusInternalServerError)
			return
		}
		shaHash := sha256.New().Sum([]byte(password))
		passw, err := hex.DecodeString(userPasswordHash)
		if err != nil {
			http.Error(w, "failed to retrieve password hash", http.StatusInternalServerError)
			return
		}
		err = bcrypt.CompareHashAndPassword(passw, shaHash[:])
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
		r.ParseForm()
		userID := r.Context().Value("userID").(uint)
		amount := r.Form.Get("amount")

		err := userRepo.DebitBalance(userID, amount)
		if err != nil {
			http.Error(w, "failed to debit balance", http.StatusInternalServerError)
			return
		}
		user, err := userRepo.GetUserByID(r.Context(), userID) // récupère le solde mis à jour
		if err != nil {
			http.Error(w, "failed to retrieve user", http.StatusInternalServerError)
			return
		}
		userRepo.InvalidateUser(userID) // invalide le cache de l'utilisateur pour forcer une mise à jour du solde
		amountValue := itoa.Atoi(amount)
		mobileMoneyClient.SendMoney(user.PhoneNumber, float64(amountValue), "mtn")
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
