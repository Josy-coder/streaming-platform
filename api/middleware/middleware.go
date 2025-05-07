package middleware

import 
import (
	"context"
	_ "errors"
	"net/http"
	"strings"
	_ "time"
A
)
type AuthService interface {
	ValidateToken(token string) (int64, string, error)
u
}
func Authenticate(authService AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}
			parts := strings.Split(auth, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Authorization header must be in the format: Bearer {token}", http.StatusUnauthorized)
				return
			}
			userId, username, err := authService.ValidateToken(parts[1])
			if err != nil {
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), "userId", userId)
			ctx = context.WithValue(ctx, "username", username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
