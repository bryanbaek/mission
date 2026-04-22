package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/bryanbaek/mission/internal/controlplane/handler"
)

func TestRegisterFrontendRoutesPreservesBackendRoutes(t *testing.T) {
	t.Parallel()

	frontend := newRouterTestFrontendHandler(t)
	router := chi.NewRouter()
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	router.Post("/api/debug/tenants/{tenantID}/query", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("debug-query"))
	})
	router.Post("/tenant.v1.TenantService/ListTenants", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("connect-rpc"))
	})
	registerFrontendRoutes(router, frontend)

	testCases := []struct {
		name   string
		method string
		target string
		want   string
	}{
		{
			name:   "health",
			method: http.MethodGet,
			target: "/healthz",
			want:   `{"status":"ok"}`,
		},
		{
			name:   "debug api",
			method: http.MethodPost,
			target: "/api/debug/tenants/tenant-1/query",
			want:   "debug-query",
		},
		{
			name:   "connect rpc",
			method: http.MethodPost,
			target: "/tenant.v1.TenantService/ListTenants",
			want:   "connect-rpc",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.target, nil)
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if body := rec.Body.String(); body != tc.want {
				t.Fatalf("body = %q, want %q", body, tc.want)
			}
		})
	}
}

func TestRegisterFrontendRoutesServesSPAAndAssets(t *testing.T) {
	t.Parallel()

	frontend := newRouterTestFrontendHandler(t)
	router := chi.NewRouter()
	registerFrontendRoutes(router, frontend)

	testCases := []struct {
		name          string
		target        string
		wantContains  string
		wantCode      int
		wantNotEquals string
	}{
		{
			name:         "chat route",
			target:       "/chat",
			wantContains: "Mission Cloud Run SPA",
			wantCode:     http.StatusOK,
		},
		{
			name:         "onboarding route",
			target:       "/onboarding/tenant-1/step-2",
			wantContains: "Mission Cloud Run SPA",
			wantCode:     http.StatusOK,
		},
		{
			name:         "asset route",
			target:       "/assets/app.js",
			wantContains: "bundled asset",
			wantCode:     http.StatusOK,
		},
		{
			name:     "reserved api prefix is not shadowed",
			target:   "/api/unknown",
			wantCode: http.StatusNotFound,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			router.ServeHTTP(rec, req)

			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantCode)
			}
			if tc.wantContains != "" && !strings.Contains(rec.Body.String(), tc.wantContains) {
				t.Fatalf("body = %q, want substring %q", rec.Body.String(), tc.wantContains)
			}
		})
	}
}

func newRouterTestFrontendHandler(t *testing.T) http.Handler {
	t.Helper()

	root := t.TempDir()
	assetsDir := filepath.Join(root, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "index.html"),
		[]byte("<!doctype html><html><body>Mission Cloud Run SPA</body></html>"),
		0o644,
	); err != nil {
		t.Fatalf("WriteFile index: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(assetsDir, "app.js"),
		[]byte("console.log('bundled asset');"),
		0o644,
	); err != nil {
		t.Fatalf("WriteFile asset: %v", err)
	}

	frontend, err := handler.NewFrontendHandler(root)
	if err != nil {
		t.Fatalf("NewFrontendHandler: %v", err)
	}
	return frontend
}
