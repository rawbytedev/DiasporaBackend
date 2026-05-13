package handlers

import (
	"Diaspora/internal/db"
	"Diaspora/internal/models"
	"Diaspora/internal/repository"
	"Diaspora/internal/solana"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

func ClaimTransfer(transferRepo *repository.TransferRepo, userRepo *repository.UserRepo, solClient *solana.Client, db *db.PostgresDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		transferIDStr := r.URL.Query().Get("Id")
		transferID, err := strconv.ParseUint(transferIDStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid transfer id", http.StatusBadRequest)
			return
		}
		userID := r.Context().Value("userID").(uint)
		// obtenir le transfert depuis DB
		var transfer models.Transfer
		if err := db.GetPool().QueryRow(r.Context(), "SELECT id, sender_id, recipient_id, amount_usdt, fees_usdt, solana_tx_hash, status, expires_at FROM transfers WHERE id = $1", transferID).Scan(&transfer.ID, &transfer.SenderID, &transfer.RecipientID, &transfer.AmountUSDT, &transfer.FeesUSDT, &transfer.SolanaTxHash, &transfer.Status, &transfer.ExpiresAt); err != nil {
			http.Error(w, "transfer not found", http.StatusNotFound)
			return
		}
		if transfer.RecipientID != userID {
			http.Error(w, "unauthorized", http.StatusForbidden)
			return
		}
		if transfer.Status != "pending" {
			http.Error(w, "already claimed or refunded", http.StatusBadRequest)
			return
		}
		if time.Now().After(transfer.ExpiresAt) {
			http.Error(w, "transfer expired", http.StatusBadRequest)
			return
		}
		// appeler smart contract claim
		err = solClient.ClaimTransfer(transfer.SolanaTxHash)
		if err != nil {
			http.Error(w, "blockchain claim failed", http.StatusInternalServerError)
			return
		}
		now := time.Now()
		if err := transferRepo.UpdateTransferStatus(uint(transferID), "claimed", &now); err != nil {
			http.Error(w, "db update failed", http.StatusInternalServerError)
			return
		}
		_ = userRepo.UpdateStateVersion(userID)
		_ = userRepo.UpdateStateVersion(transfer.SenderID)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "claimed"})
	}
}
