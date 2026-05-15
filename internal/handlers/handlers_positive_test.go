package handlers_test

import (
	"Diaspora/internal/handlers"
	"Diaspora/internal/middleware"
	"Diaspora/internal/mocks"
	"Diaspora/internal/repository"
	"Diaspora/internal/testhelpers"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gagliardetto/solana-go"
)

func TestSendTransfer_HappyPath(t *testing.T) {
	tc := testhelpers.SetupTestContext(t)
	defer tc.Close()
	defer tc.TruncateAllTables()

	solMock := mocks.NewMockSolanaClient()
	userRepo := repository.NewUserRepo(tc.Cache, tc.DB.PostgresDB, solMock)
	transferRepo := repository.NewTransferRepo(tc.Cache, tc.DB.PostgresDB)

	sender, err := tc.CreateTestUser("+229700000001", "Sender", solana.NewWallet().PrivateKey)
	if err != nil {
		t.Fatalf("CreateTestUser failed: %v", err)
	}
	recipient, err := tc.CreateTestUser("+229700000002", "Recipient", solana.NewWallet().PrivateKey)
	if err != nil {
		t.Fatalf("CreateTestUser failed: %v", err)
	}

	body := map[string]interface{}{
		"recipient_phone": recipient.PhoneNumber,
		"amount_usdt":     100.0,
	}
	req := newJSONReq(t, http.MethodPost, "/api/transfer", body)
	token, err := middleware.GenerateToken(sender.ID)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	handler := middleware.AuthMiddleware(handlers.SendTransfer(userRepo, transferRepo, solMock))
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Result().Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["tx_hash"] == nil {
		t.Fatal("expected tx_hash in response")
	}
}

func TestWithdraw_HappyPath(t *testing.T) {
	tc := testhelpers.SetupTestContext(t)
	defer tc.Close()
	defer tc.TruncateAllTables()

	solMock := mocks.NewMockSolanaClient()
	solMock.SetMockBalance(1000)
	userRepo := repository.NewUserRepo(tc.Cache, tc.DB.PostgresDB, solMock)
	mmMock := mocks.NewMockMobileMoneyClient()

	user, err := tc.CreateTestUser("+229700000003", "Withdrawer", solana.NewWallet().PrivateKey)
	if err != nil {
		t.Fatalf("CreateTestUser failed: %v", err)
	}

	body := map[string]interface{}{
		"amount_usdt": 10.0,
		"provider":    "mtn",
	}
	req := newJSONReq(t, http.MethodPost, "/api/withdraw", body)
	token, err := middleware.GenerateToken(user.ID)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	handler := middleware.AuthMiddleware(handlers.Withdraw(userRepo, mmMock))
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Result().Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["mobile_tx_id"] == nil {
		t.Fatal("expected mobile_tx_id in response")
	}
}
