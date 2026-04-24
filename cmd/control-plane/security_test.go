package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaxBodySizeRejectsOversizedRequests(t *testing.T) {
	t.Parallel()

	const limit = 16

	handler := maxBodySize(limit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))

	t.Run("small body succeeds", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("hello"))
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if rec.Body.String() != "hello" {
			t.Fatalf("body = %q, want %q", rec.Body.String(), "hello")
		}
	})

	t.Run("oversized body rejected", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		oversized := strings.Repeat("x", limit+1)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(oversized))
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413", rec.Code)
		}
	})
}

func TestMaxBodySizeAllowsExactLimit(t *testing.T) {
	t.Parallel()

	const limit = 10

	handler := maxBodySize(limit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))

	rec := httptest.NewRecorder()
	exact := strings.Repeat("x", limit)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(exact))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
