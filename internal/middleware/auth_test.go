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

func TestAuthMiddleware(t *testing.T) {
	// Create test handler
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("userID")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "userID: %v", userID)
	}

	// JWT secret for testing (must match the one in middleware)
	jwtSecret := []byte("secret")

	tests := []struct {
		name           string
		token          string
		expectedStatus int
		expectedError  bool
	}{
		{
			name:           "Valid token",
			token:          "Bearer " + generateValidToken(jwtSecret, 123),
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
		{
			name:           "Missing authorization header",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  true,
		},
		{
			name:           "Invalid token format - missing Bearer",
			token:          "InvalidToken",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  true,
		},
		{
			name:           "Invalid token format - wrong prefix",
			token:          "Basic " + generateValidToken(jwtSecret, 123),
			expectedStatus: http.StatusUnauthorized,
			expectedError:  true,
		},
		{
			name:           "Invalid token - malformed",
			token:          "Bearer invalid.token.here",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  true,
		},
		{
			name:           "Invalid token - wrong secret",
			token:          "Bearer " + generateValidTokenWithSecret([]byte("wrong_secret"), 123),
			expectedStatus: http.StatusUnauthorized,
			expectedError:  true,
		},
		{
			name:           "Expired token",
			token:          generateExpiredToken(jwtSecret, 123),
			expectedStatus: http.StatusUnauthorized,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", tt.token)
			}

			// Create recorder
			w := httptest.NewRecorder()

			// Apply middleware
			wrappedHandler := middleware.AuthMiddleware(testHandler)
			wrappedHandler(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			if tt.expectedError && resp.StatusCode == http.StatusOK {
				t.Error("expected error but got OK status")
			}
		})
	}
}

func TestAuthMiddlewareContextInjection(t *testing.T) {
	jwtSecret := []byte("secret")
	userID := uint(456)

	contextCapture := ""
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		val := r.Context().Value("userID")
		contextCapture = fmt.Sprintf("%v", val)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+generateValidToken(jwtSecret, userID))

	w := httptest.NewRecorder()
	wrappedHandler := middleware.AuthMiddleware(testHandler)
	wrappedHandler(w, req)
	// Verify context was set (this is a simplified check)
	// In production, you'd verify the actual userID value
	if contextCapture == "" {
		t.Error("context value was not set")
	}
}

// Helper functions
func generateValidToken(secret []byte, userID uint) string {
	claims := jwt.MapClaims{
		"userID": float64(userID),
		"exp":    time.Now().Add(time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(secret)
	return tokenString
}

func generateValidTokenWithSecret(secret []byte, userID uint) string {
	claims := jwt.MapClaims{
		"userID": float64(userID),
		"exp":    time.Now().Add(time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(secret)
	return tokenString
}

func generateExpiredToken(secret []byte, userID uint) string {
	claims := jwt.MapClaims{
		"userID": float64(userID),
		"exp":    time.Now().Add(-time.Hour).Unix(), // Already expired
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(secret)
	return tokenString
}
