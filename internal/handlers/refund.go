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

// RefundTransfer allows the original sender to reclaim escrowed tokens after
// the 7-day expiry window has passed.
//
//	POST /api/refund
//	Authorization: Bearer <token>
//	Content-Type: application/json
//	Body: {"transfer_id": 42}
func RefundTransfer(transferRepo *repository.TransferRepo, userRepo *repository.UserRepo, solClient solana.ClientInterface, _ *db.PostgresDB) http.HandlerFunc {
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

		if transfer.SenderID != userID {
			middleware.JSONError(w, http.StatusForbidden, "only the sender can request a refund")
			return
		}
		if transfer.Status != "pending" {
			middleware.JSONError(w, http.StatusBadRequest,
				"transfer cannot be refunded (status: "+transfer.Status+")")
			return
		}
		if time.Now().Before(transfer.ExpiresAt) {
			remaining := time.Until(transfer.ExpiresAt).Round(time.Minute)
			middleware.JSONError(w, http.StatusBadRequest,
				"transfer has not expired yet; refund available in "+remaining.String())
			return
		}

		if err := solClient.RefundTransfer(transfer.SolanaTxHash); err != nil {
			middleware.JSONError(w, http.StatusInternalServerError, "blockchain refund failed: "+err.Error())
			return
		}

		if err := transferRepo.UpdateTransferStatus(r.Context(), transfer.ID, "refunded", nil); err != nil {
			middleware.JSONError(w, http.StatusInternalServerError, "failed to update transfer record")
			return
		}

		_ = userRepo.UpdateStateVersion(userID)
		_ = userRepo.UpdateStateVersion(transfer.RecipientID)

		middleware.JSONResponse(w, http.StatusOK, map[string]interface{}{
			"status":      "refunded",
			"tx_hash":     transfer.SolanaTxHash,
			"transfer_id": transfer.ID,
		})
	}
}
