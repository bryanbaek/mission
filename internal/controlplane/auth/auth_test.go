package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFakeVerifier(t *testing.T) {
	t.Parallel()

	verifier := &FakeVerifier{
		Tokens: map[string]User{
			"good-token": {ID: "user_123"},
		},
	}

	user, err := verifier.Verify(context.Background(), "good-token")
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if user.ID != "user_123" {
		t.Fatalf("Verify returned wrong user id: %q", user.ID)
	}

	_, err = verifier.Verify(context.Background(), "bad-token")
	if err == nil {
		t.Fatal("Verify returned nil error for invalid token")
	}
}

func TestWithUserAndFromContext(t *testing.T) {
	t.Parallel()

	ctx := WithUser(context.Background(), User{ID: "user_456"})
	user, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext did not find user")
	}
	if user.ID != "user_456" {
		t.Fatalf("FromContext returned wrong user id: %q", user.ID)
	}

	_, ok = FromContext(context.Background())
	if ok {
		t.Fatal("FromContext unexpectedly found a user in empty context")
	}
}

func TestRequireAuthMissingBearerToken(t *testing.T) {
	t.Parallel()

	handler := RequireAuth(&FakeVerifier{})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler should not run without bearer token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content type = %q, want application/json", rec.Header().Get("Content-Type"))
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body["error"] != "missing bearer token" {
		t.Fatalf("error body = %q, want missing bearer token", body["error"])
	}
}

func TestRequireAuthInvalidToken(t *testing.T) {
	t.Parallel()

	handler := RequireAuth(&FakeVerifier{})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler should not run with invalid token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body["error"] != "invalid token" {
		t.Fatalf("error body = %q, want invalid token", body["error"])
	}
}

func TestRequireAuthInjectsVerifiedUser(t *testing.T) {
	t.Parallel()

	verifier := &FakeVerifier{
		Tokens: map[string]User{
			"good-token": {ID: "user_789"},
		},
	}

	handler := RequireAuth(verifier)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := FromContext(r.Context())
		if !ok {
			t.Fatal("expected authenticated user in context")
		}
		if user.ID != "user_789" {
			t.Fatalf("context user id = %q, want user_789", user.ID)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestWriteJSONErrorWritesJSON(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	writeJSONError(rec, http.StatusTeapot, "brew failed")

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content type = %q, want application/json", rec.Header().Get("Content-Type"))
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body["error"] != "brew failed" {
		t.Fatalf("error body = %q, want brew failed", body["error"])
	}
}
