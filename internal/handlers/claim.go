package handlers

import (
	"Diaspora/internal/db"
	"Diaspora/internal/middleware"
	"Diaspora/internal/repository"
	"Diaspora/internal/solana"
	"encoding/json"
	"net/http"
	"time"
)

// ClaimTransfer allows the recipient to claim a pending escrow transfer.
//
//	POST /api/claim
//	Authorization: Bearer <token>
//	Content-Type: application/json
//	Body: {"transfer_id": 42}
func ClaimTransfer(transferRepo *repository.TransferRepo, userRepo *repository.UserRepo, solClient solana.ClientInterface, _ *db.PostgresDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			middleware.JSONError(w, http.StatusUnauthorized, "missing user context")
			return
		}

		var body struct {
			TransferID uint `json:"transfer_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TransferID == 0 {
			middleware.JSONError(w, http.StatusBadRequest, "transfer_id is required")
			return
		}

		transfer, err := transferRepo.GetTransferByID(r.Context(), body.TransferID)
		if err != nil {
			middleware.JSONError(w, http.StatusNotFound, "transfer not found")
			return
		}

		if transfer.RecipientID != userID {
			middleware.JSONError(w, http.StatusForbidden, "only the recipient can claim this transfer")
			return
		}
		if transfer.Status != "pending" {
			middleware.JSONError(w, http.StatusBadRequest,
				"transfer cannot be claimed (status: "+transfer.Status+")")
			return
		}
		if time.Now().After(transfer.ExpiresAt) {
			middleware.JSONError(w, http.StatusBadRequest, "transfer has expired; sender may now request a refund")
			return
		}

		if err := solClient.ClaimTransfer(transfer.SolanaTxHash); err != nil {
			middleware.JSONError(w, http.StatusInternalServerError, "blockchain claim failed: "+err.Error())
			return
		}

		now := time.Now()
		if err := transferRepo.UpdateTransferStatus(r.Context(), transfer.ID, "claimed", &now); err != nil {
			middleware.JSONError(w, http.StatusInternalServerError, "failed to update transfer record")
			return
		}

		_ = userRepo.UpdateStateVersion(userID)
		_ = userRepo.UpdateStateVersion(transfer.SenderID)

		middleware.JSONResponse(w, http.StatusOK, map[string]interface{}{
			"status":      "claimed",
			"tx_hash":     transfer.SolanaTxHash,
			"amount_usdt": transfer.AmountUSDT,
			"claimed_at":  now,
		})
	}
}
