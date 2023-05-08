package auth

import (
	"net/http"
	"strings"
)

func ApiKeyAuthMiddleware(apiKey string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == apiKey {
				next.ServeHTTP(w, r)
			} else {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
		})
	}
}
