// Package auth provides JWT verification + middleware that injects a verified
// user identity into request context. Real verification uses Clerk; tests use
// the in-memory FakeVerifier.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
)

// User represents the verified caller of an API request.
type User struct {
	ID string // Clerk user id (sub claim, e.g. "user_2abc...")
}

// Verifier verifies a bearer token and returns the authenticated user.
type Verifier interface {
	Verify(ctx context.Context, token string) (User, error)
}

// ClerkVerifier verifies tokens against Clerk. Call clerk.SetKey before use.
type ClerkVerifier struct{}

func NewClerkVerifier(secretKey string) *ClerkVerifier {
	clerk.SetKey(secretKey)
	return &ClerkVerifier{}
}

func (ClerkVerifier) Verify(ctx context.Context, token string) (User, error) {
	claims, err := jwt.Verify(ctx, &jwt.VerifyParams{Token: token})
	if err != nil {
		return User{}, err
	}
	return User{ID: claims.Subject}, nil
}

// FakeVerifier is for tests. Its map is token-string → User.
type FakeVerifier struct {
	Tokens map[string]User
}

func (f *FakeVerifier) Verify(_ context.Context, token string) (User, error) {
	if u, ok := f.Tokens[token]; ok {
		return u, nil
	}
	return User{}, errors.New("invalid token")
}

type ctxKey int

const userKey ctxKey = iota

// WithUser stores u in ctx (test-only convenience; production goes through middleware).
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// FromContext returns the authenticated user, or false if missing.
func FromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userKey).(User)
	return u, ok
}

// RequireAuth returns chi-compatible middleware that rejects requests without
// a valid bearer token and injects the verified User into the request context.
func RequireAuth(v Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			if !strings.HasPrefix(authz, "Bearer ") {
				writeJSONError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			token := strings.TrimPrefix(authz, "Bearer ")
			user, err := v.Verify(r.Context(), token)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "invalid token")
				return
			}
			ctx := WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
