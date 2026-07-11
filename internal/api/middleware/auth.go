package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// RequireAuth ngecek header "Authorization: Bearer {token}" cocok sama
// AuthToken yang di-set di config.json (sama persis kayak daemon_token
// yang Panel generate pas bikin Node).
//
// Pakai subtle.ConstantTimeCompare biar nggak kena timing attack pas
// bandingin token.
func RequireAuth(expectedToken string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")

		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			http.Error(w, `{"error":"missing or malformed Authorization header"}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(header, prefix)

		if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
