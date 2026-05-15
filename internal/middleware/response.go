// Package middleware provides shared HTTP helpers for JSON responses and errors.
package middleware

import (
	"encoding/json"
	"net/http"
)

// JSONResponse writes a JSON payload with the given status code.
func JSONResponse(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

// JSONError writes a structured {"error": "..."} JSON body with the given status code.
func JSONError(w http.ResponseWriter, status int, message string) {
	JSONResponse(w, status, map[string]string{"error": message})
}
