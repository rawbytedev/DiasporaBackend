package handlers

import (
	"Diaspora/internal/middleware"
	"Diaspora/internal/repository"
	"Diaspora/internal/solana"
	"net/http"
)

// GetAccount returns the authenticated user's profile information.
//
//	GET /api/account
//	Authorization: Bearer <token>
func GetAccount(userRepo *repository.UserRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			middleware.JSONError(w, http.StatusUnauthorized, "missing user context")
			return
		}

		user, err := userRepo.GetUserByID(r.Context(), userID)
		if err != nil {
			middleware.JSONError(w, http.StatusNotFound, "user not found")
			return
		}

		middleware.JSONResponse(w, http.StatusOK, map[string]interface{}{
			"id":            user.ID,
			"name":          user.Name,
			"phone_number":  user.PhoneNumber,
			"solana_pubkey": user.SolanaPubkey,
			"created_at":    user.CreatedAt,
		})
	}
}

// GetBalance returns the authenticated user's USDT balance on Solana.
//
//	GET /api/balance
//	Authorization: Bearer <token>
func GetBalance(userRepo *repository.UserRepo, solClient solana.ClientInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			middleware.JSONError(w, http.StatusUnauthorized, "missing user context")
			return
		}

		user, err := userRepo.GetUserByID(r.Context(), userID)
		if err != nil {
			middleware.JSONError(w, http.StatusNotFound, "user not found")
			return
		}

		balance, err := solClient.GetTokenBalance(user.SolanaPubkey)
		if err != nil {
			// Balance may be unavailable when the Solana node is unreachable or
			// the user has no USDT token account yet.  Return 0 with context.
			middleware.JSONResponse(w, http.StatusOK, map[string]interface{}{
				"solana_pubkey": user.SolanaPubkey,
				"balance_usdt":  0.0,
				"warning":       "could not fetch live balance: " + err.Error(),
			})
			return
		}

		middleware.JSONResponse(w, http.StatusOK, map[string]interface{}{
			"solana_pubkey": user.SolanaPubkey,
			"balance_usdt":  balance,
		})
	}
}
