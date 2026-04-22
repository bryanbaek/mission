package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFrontendRuntimeConfigHandlerJSON(t *testing.T) {
	t.Parallel()

	handler := NewFrontendRuntimeConfigHandler(FrontendRuntimeConfig{
		ClerkPublishableKey: "pk_test_123",
		SentryDSN:           "https://public@example.com/1",
		SentryEnvironment:   "production",
		SentryRelease:       "release-1",
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/app-config.json", nil)
	handler.JSON(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("content type = %q, want application/json", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache control = %q, want no-store", got)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"clerkPublishableKey":"pk_test_123"`,
		`"sentryDsn":"https://public@example.com/1"`,
		`"sentryEnvironment":"production"`,
		`"sentryRelease":"release-1"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body = %q, want substring %q", body, want)
		}
	}
}
