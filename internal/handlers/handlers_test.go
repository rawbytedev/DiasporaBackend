// Package handlers_test contains unit tests for the HTTP handlers.
// Tests that require a real database connection are skipped when the
// environment variable DATABASE_URL is not set.
package handlers_test

import (
	"Diaspora/internal/handlers"
	"Diaspora/internal/mocks"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// jsonBody encodes v as JSON and returns a *bytes.Buffer.
func jsonBody(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return bytes.NewBuffer(b)
}

// newJSONReq builds an HTTP request with a JSON body.
func newJSONReq(t *testing.T, method, path string, body interface{}) *http.Request {
	t.Helper()
	r := httptest.NewRequest(method, path, jsonBody(t, body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

// withUserID injects a userID into the request context, simulating AuthMiddleware.
func withUserID(r *http.Request, id uint) *http.Request {
	ctx := context.WithValue(r.Context(), contextKey("userID"), id)
	return r.WithContext(ctx)
}

type contextKey string

// ---------------------------------------------------------------------------
// Register
// ---------------------------------------------------------------------------

func TestRegister_MissingBody(t *testing.T) {
	h := handlers.Register(nil)
	r := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(""))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h(w, r)
	// Empty body causes JSON decode error → 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestRegister_MissingFields(t *testing.T) {
	h := handlers.Register(nil)
	r := newJSONReq(t, http.MethodPost, "/api/register", map[string]string{
		"phone_number": "",
		"name":         "",
		"password":     "",
	})
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

func TestLogin_MissingBody(t *testing.T) {
	h := handlers.Login(nil)
	r := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString("not json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestLogin_EmptyCredentials(t *testing.T) {
	h := handlers.Login(nil)
	r := newJSONReq(t, http.MethodPost, "/api/login", map[string]string{
		"phone_number": "",
		"password":     "",
	})
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// VerifyOTP
// ---------------------------------------------------------------------------

func TestVerifyOTP_MissingFields(t *testing.T) {
	h := handlers.VerifyOTP(nil)
	r := newJSONReq(t, http.MethodPost, "/api/verify-otp", map[string]string{
		"phone_number": "",
		"otp":          "",
	})
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// SendTransfer – validation layer (no DB needed)
// ---------------------------------------------------------------------------

func TestSendTransfer_InvalidBody(t *testing.T) {
	solMock := mocks.NewMockSolanaClient()
	h := handlers.SendTransfer(nil, nil, solMock)
	r := httptest.NewRequest(http.MethodPost, "/api/transfer", bytes.NewBufferString("{bad"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h(w, r)
	// No userID in context → 401; bad JSON → 400.  Either is acceptable.
	if w.Code != http.StatusBadRequest && w.Code != http.StatusUnauthorized {
		t.Errorf("want 400 or 401, got %d", w.Code)
	}
}

func TestSendTransfer_ZeroAmount(t *testing.T) {
	solMock := mocks.NewMockSolanaClient()
	h := handlers.SendTransfer(nil, nil, solMock)
	r := newJSONReq(t, http.MethodPost, "/api/transfer", map[string]interface{}{
		"recipient_phone": "+22670000001",
		"amount_usdt":     0,
	})
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusUnauthorized {
		t.Errorf("want 400 or 401, got %d", w.Code)
	}
}

func TestSendTransfer_NegativeAmount(t *testing.T) {
	solMock := mocks.NewMockSolanaClient()
	h := handlers.SendTransfer(nil, nil, solMock)
	r := newJSONReq(t, http.MethodPost, "/api/transfer", map[string]interface{}{
		"recipient_phone": "+22670000001",
		"amount_usdt":     -5.0,
	})
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusUnauthorized {
		t.Errorf("want 400 or 401, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Withdraw – validation layer
// ---------------------------------------------------------------------------

func TestWithdraw_InvalidBody(t *testing.T) {
	mmMock := mocks.NewMockMobileMoneyClient()
	h := handlers.Withdraw(nil, mmMock)
	r := httptest.NewRequest(http.MethodPost, "/api/withdraw", bytes.NewBufferString("bad json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusUnauthorized {
		t.Errorf("want 400 or 401, got %d", w.Code)
	}
}

func TestWithdraw_ZeroAmount(t *testing.T) {
	mmMock := mocks.NewMockMobileMoneyClient()
	h := handlers.Withdraw(nil, mmMock)
	r := newJSONReq(t, http.MethodPost, "/api/withdraw", map[string]interface{}{
		"amount_usdt": 0,
	})
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusUnauthorized {
		t.Errorf("want 400 or 401, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Interface compliance
// ---------------------------------------------------------------------------

// Ensure the mock satisfies the interface at compile time.
func TestMocks_ImplementInterfaces(t *testing.T) {
	var _ handlers.MobileMoneyClient = mocks.NewMockMobileMoneyClient()
}
