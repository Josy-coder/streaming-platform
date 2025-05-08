package middleware

import (
	"context"
	_ "errors"
	"net/http"
	"strings"
	_ "time"
)

// AuthService is the interface for authentication services
type AuthService interface {
	ValidateToken(token string) (int64, string, error)
}

// Authenticate middleware checks for a valid token
func Authenticate(authService AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get token from Authorization header
			auth := r.Header.Get("Authorization")
			if auth == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			// Check Bearer format
			parts := strings.Split(auth, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Authorization header must be in the format: Bearer {token}", http.StatusUnauthorized)
				return
			}

			// Validate token
			userId, username, err := authService.ValidateToken(parts[1])
			if err != nil {
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Add user info to context
			ctx := context.WithValue(r.Context(), "userId", userId)
			ctx = context.WithValue(ctx, "username", username)

			// Call the next handler with the updated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
