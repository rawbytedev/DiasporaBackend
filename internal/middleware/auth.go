// Package middleware provides HTTP middleware for the DiasporaConnect API.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// jwtSecret is loaded from the JWT_SECRET environment variable.
// A hardcoded fallback is used only in development so the app starts without
// configuration; production deployments must set the variable explicitly.
var jwtSecret = func() []byte {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return []byte(s)
	}
	return []byte("diaspora-dev-secret-change-in-prod")
}()

// contextKey is an unexported type used for context keys to avoid collisions.
type contextKey string

const userIDKey contextKey = "userID"

// AuthMiddleware validates the JWT Bearer token and injects the userID into
// the request context.  All protected handlers read the user identity with
// UserIDFromContext.
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			JSONError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			JSONError(w, http.StatusUnauthorized, "invalid authorization format, expected 'Bearer <token>'")
			return
		}

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(parts[1], claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			JSONError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		userID, err := parseUserIDFromClaims(claims)
		if err != nil {
			JSONError(w, http.StatusUnauthorized, "invalid userID claim")
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next(w, r.WithContext(ctx))
	}
}

// UserIDFromContext extracts the authenticated user ID set by AuthMiddleware.
func UserIDFromContext(ctx context.Context) (uint, bool) {
	v, ok := ctx.Value(userIDKey).(uint)
	return v, ok
}

// GenerateToken creates a signed JWT for the given user ID with a 24-hour expiry.
func GenerateToken(userID uint) (string, error) {
	claims := jwt.MapClaims{
		"userID": userID,
		"exp":    time.Now().Add(24 * time.Hour).Unix(),
		"iat":    time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// parseUserIDFromClaims handles the float64 that JWT parsers typically use for
// numeric claims, as well as string and integer fallbacks.
func parseUserIDFromClaims(claims jwt.MapClaims) (uint, error) {
	value, ok := claims["userID"]
	if !ok {
		return 0, fmt.Errorf("missing userID claim")
	}

	switch v := value.(type) {
	case float64:
		return uint(v), nil
	case float32:
		return uint(v), nil
	case int:
		return uint(v), nil
	case int64:
		return uint(v), nil
	case uint:
		return v, nil
	case uint64:
		return uint(v), nil
	case string:
		u64, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, err
		}
		return uint(u64), nil
	default:
		return 0, fmt.Errorf("unsupported userID claim type %T", value)
	}
}
