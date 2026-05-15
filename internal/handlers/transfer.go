package handlers

import (
	"Diaspora/internal/middleware"
	"Diaspora/internal/models"
	"Diaspora/internal/repository"
	"Diaspora/internal/solana"
	"encoding/json"
	"net/http"
	"time"
)

// TransferRequest is the JSON body for POST /api/transfer.
type TransferRequest struct {
	RecipientPhone string  `json:"recipient_phone"`
	AmountUSDT     float64 `json:"amount_usdt"`
}

// SendTransfer initiates a USDT escrow transfer on the Solana blockchain.
//
//	POST /api/transfer
//	Authorization: Bearer <token>
//	Content-Type: application/json
func SendTransfer(userRepo *repository.UserRepo, transferRepo *repository.TransferRepo, solClient solana.ClientInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			middleware.JSONError(w, http.StatusUnauthorized, "missing user context")
			return
		}

		var req TransferRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			middleware.JSONError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.RecipientPhone == "" {
			middleware.JSONError(w, http.StatusBadRequest, "recipient_phone is required")
			return
		}
		if req.AmountUSDT <= 0 {
			middleware.JSONError(w, http.StatusBadRequest, "amount_usdt must be positive")
			return
		}

		recipient, err := userRepo.GetUserByPhone(r.Context(), req.RecipientPhone)
		if err != nil {
			middleware.JSONError(w, http.StatusNotFound, "recipient not found")
			return
		}
		if recipient.ID == userID {
			middleware.JSONError(w, http.StatusBadRequest, "cannot transfer to yourself")
			return
		}

		// 1 % platform fee.
		fees := req.AmountUSDT * 0.01
		netAmount := req.AmountUSDT - fees

		txHash, err := solClient.InitiateTransfer(userID, recipient.ID, netAmount, fees)
		if err != nil {
			middleware.JSONError(w, http.StatusInternalServerError, "blockchain error: "+err.Error())
			return
		}

		transfer := &models.Transfer{
			SenderID:     userID,
			RecipientID:  recipient.ID,
			AmountUSDT:   netAmount,
			FeesUSDT:     fees,
			SolanaTxHash: txHash,
			Status:       "pending",
			ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
		}
		if err := transferRepo.CreateTransfer(r.Context(), transfer); err != nil {
			middleware.JSONError(w, http.StatusInternalServerError, "failed to save transfer record")
			return
		}

		_ = userRepo.UpdateStateVersion(userID)
		_ = userRepo.UpdateStateVersion(recipient.ID)
		_ = transferRepo.InvalidateTransferCaches(userID, recipient.ID, userRepo)

		middleware.JSONResponse(w, http.StatusCreated, map[string]interface{}{
			"transfer_id":     transfer.ID,
			"tx_hash":         txHash,
			"amount_usdt":     netAmount,
			"fees_usdt":       fees,
			"recipient_phone": req.RecipientPhone,
			"status":          "pending",
			"expires_at":      transfer.ExpiresAt,
		})
	}
}
