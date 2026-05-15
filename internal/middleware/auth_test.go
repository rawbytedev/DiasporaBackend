package middleware_test

import (
	"Diaspora/internal/middleware"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// wrongSecret is deliberately different from the package-level secret so we
// can test that tokens signed with a different key are rejected.
var wrongSecret = []byte("wrong-secret-key")

// generateTokenWithSecret creates a JWT signed with an arbitrary secret.
// Use this only to produce intentionally-invalid tokens in tests.
func generateTokenWithSecret(secret []byte, userID uint, exp time.Duration) string {
	claims := jwt.MapClaims{
		"userID": float64(userID),
		"exp":    time.Now().Add(exp).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := token.SignedString(secret)
	return s
}

func TestAuthMiddleware(t *testing.T) {
	// validToken is produced by the middleware's own GenerateToken so it uses
	// the same secret that AuthMiddleware will validate against.
	validToken, err := middleware.GenerateToken(123)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "Valid token",
			authHeader:     "Bearer " + validToken,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Missing authorization header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid format – no Bearer prefix",
			authHeader:     "InvalidToken",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid format – wrong scheme",
			authHeader:     "Basic " + validToken,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Malformed token",
			authHeader:     "Bearer not.a.jwt",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Wrong signing secret",
			authHeader:     "Bearer " + generateTokenWithSecret(wrongSecret, 123, time.Hour),
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Expired token",
			authHeader:     "Bearer " + generateTokenWithSecret([]byte("diaspora-dev-secret-change-in-prod"), 123, -time.Hour),
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()
			handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("want %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestAuthMiddlewareContextInjection(t *testing.T) {
	wantID := uint(456)

	token, err := middleware.GenerateToken(wantID)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	var gotID uint
	var ok bool

	handler := middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		gotID, ok = middleware.UserIDFromContext(r.Context())
		fmt.Fprintf(w, "%d", gotID)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if !ok {
		t.Fatal("UserIDFromContext returned ok=false; context value was not set")
	}
	if gotID != wantID {
		t.Errorf("userID: want %d, got %d", wantID, gotID)
	}
}
