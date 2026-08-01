// Package auth is the desktop↔backend session token: generated at launch,
// held in memory only, required on every REST request and the first WebSocket
// message.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

// Token is an opaque bearer credential.
type Token string

// Generate returns a new 256-bit random token, hex encoded.
func Generate() (Token, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generating session token: %w", err)
	}
	return Token(hex.EncodeToString(b[:])), nil
}

// Valid performs a constant-time comparison.
func (t Token) Valid(guess string) bool {
	if t == "" || guess == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(t), []byte(guess)) == 1
}

// FromRequest extracts a bearer token from the Authorization header.
func FromRequest(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
}

// Middleware enforces bearer authentication on every request.
func (t Token) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !t.Valid(FromRequest(r)) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="openinfer"`)
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
