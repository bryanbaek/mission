package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFrontendHandlerServesIndexForSPARoutes(t *testing.T) {
	t.Parallel()

	handler := newTestFrontendHandler(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/chat", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "Mission SPA") {
		t.Fatalf("body = %q, want Mission SPA index", body)
	}
}

func TestFrontendHandlerServesStaticAssets(t *testing.T) {
	t.Parallel()

	handler := newTestFrontendHandler(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "frontend asset") {
		t.Fatalf("body = %q, want asset content", body)
	}
}

func TestFrontendHandlerReturnsNotFoundForMissingAssets(t *testing.T) {
	t.Parallel()

	handler := newTestFrontendHandler(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestFrontendHandlerReturnsNotFoundForReservedPaths(t *testing.T) {
	t.Parallel()

	handler := newTestFrontendHandler(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/debug/unknown", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func newTestFrontendHandler(t *testing.T) *FrontendHandler {
	t.Helper()

	root := t.TempDir()
	assetsDir := filepath.Join(root, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "index.html"),
		[]byte("<!doctype html><html><body>Mission SPA</body></html>"),
		0o644,
	); err != nil {
		t.Fatalf("WriteFile index: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(assetsDir, "app.js"),
		[]byte("console.log('frontend asset');"),
		0o644,
	); err != nil {
		t.Fatalf("WriteFile asset: %v", err)
	}

	handler, err := NewFrontendHandler(root)
	if err != nil {
		t.Fatalf("NewFrontendHandler: %v", err)
	}
	return handler
}
