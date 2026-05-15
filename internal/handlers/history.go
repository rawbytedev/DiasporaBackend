package handlers

import (
	"Diaspora/internal/middleware"
	"Diaspora/internal/repository"
	"net/http"
	"strconv"
)

// GetTransfers returns the authenticated user's transfer history.
//
// Query parameters:
//   - status    – filter by status: "pending" | "claimed" | "refunded" (optional)
//   - direction – "sent" | "received" | "all" (default "all")
//
//	GET /api/transfers
//	Authorization: Bearer <token>
func GetTransfers(transferRepo *repository.TransferRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			middleware.JSONError(w, http.StatusUnauthorized, "missing user context")
			return
		}

		status := r.URL.Query().Get("status")
		direction := r.URL.Query().Get("direction")

		var (
			transfers interface{}
			err       error
		)

		switch direction {
		case "sent":
			transfers, err = transferRepo.GetSentTransfers(r.Context(), userID, status)
		case "received":
			transfers, err = transferRepo.GetReceivedTransfers(r.Context(), userID, status)
		default:
			transfers, err = transferRepo.GetTransfersByUserID(r.Context(), userID, status)
		}

		if err != nil {
			middleware.JSONError(w, http.StatusInternalServerError, "failed to fetch transfers")
			return
		}

		middleware.JSONResponse(w, http.StatusOK, map[string]interface{}{
			"transfers": transfers,
		})
	}
}

// GetTransfer returns a single transfer by ID.
//
//	GET /api/transfers/detail?id=42
//	Authorization: Bearer <token>
func GetTransfer(transferRepo *repository.TransferRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			middleware.JSONError(w, http.StatusUnauthorized, "missing user context")
			return
		}

		idStr := r.URL.Query().Get("id")
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil || id == 0 {
			middleware.JSONError(w, http.StatusBadRequest, "invalid or missing transfer id")
			return
		}

		transfer, err := transferRepo.GetTransferByID(r.Context(), uint(id))
		if err != nil {
			middleware.JSONError(w, http.StatusNotFound, "transfer not found")
			return
		}

		// Only the sender or recipient may view the transfer.
		if transfer.SenderID != userID && transfer.RecipientID != userID {
			middleware.JSONError(w, http.StatusForbidden, "access denied")
			return
		}

		middleware.JSONResponse(w, http.StatusOK, transfer)
	}
}
