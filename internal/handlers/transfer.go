package handlers

import (
	"Diaspora/internal/models"
	"Diaspora/internal/repository"
	"Diaspora/internal/solana"
	"encoding/json"
	"net/http"
	"time"
)

type TransferRequest struct {
	RecipientPhone string  `json:"recipient_phone"`
	AmountUSDT     float64 `json:"amount_usdt"`
}

func SendTransfer(userRepo *repository.UserRepo, transferRepo *repository.TransferRepo, solClient *solana.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req TransferRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		senderID := r.Context().Value("userID").(uint)
		// récupérer destinataire
		recipient, err := userRepo.GetUserByPhone(r.Context(), req.RecipientPhone)
		if err != nil {
			http.Error(w, "recipient not found", http.StatusNotFound)
			return
		}
		// calculer frais 1%
		fees := req.AmountUSDT * 0.01
		netAmount := req.AmountUSDT - fees
		// construire transaction Solana (appel au smart contract)
		txHash, err := solClient.InitiateTransfer(senderID, recipient.ID, netAmount, fees)
		if err != nil {
			http.Error(w, "blockchain error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// sauvegarder en base
		transfer := &models.Transfer{
			SenderID:     senderID,
			RecipientID:  recipient.ID,
			AmountUSDT:   netAmount,
			FeesUSDT:     fees,
			SolanaTxHash: txHash,
			Status:       "pending",
			ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
		}
		if err := transferRepo.CreateTransfer(r.Context(), transfer); err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		// invalider caches
		_ = userRepo.UpdateStateVersion(senderID)
		_ = userRepo.UpdateStateVersion(recipient.ID)
		_ = transferRepo.InvalidateTransferCaches(senderID, recipient.ID, userRepo)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tx_hash": txHash,
			"status":  "pending",
		})
	}
}
