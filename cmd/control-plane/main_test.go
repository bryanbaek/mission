package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/bryanbaek/mission/internal/controlplane/config"
	"github.com/bryanbaek/mission/internal/controlplane/handler"
	"github.com/bryanbaek/mission/internal/controlplane/llmprovider"
)

func TestBuildLLMRuntimeBuildsConfiguredProvidersInCatalogOrder(t *testing.T) {
	t.Parallel()

	providers, semanticModels, queryModels, err := buildLLMRuntime(config.Config{
		DefaultLLMProvider: "deepseek",
		ProviderAPIKeys: map[string]string{
			"deepseek":    "deepseek-key",
			"anthropic":   "anthropic-key",
			"openai":      "openai-key",
			"fireworks":   "",
			"unsupported": "ignored",
		},
		SemanticLayerProviderModels: map[string]string{
			"deepseek":  "deepseek-chat",
			"anthropic": "claude-sonnet-4-6",
		},
		QueryProviderModels: map[string]string{
			"deepseek": "deepseek-chat",
			"openai":   "gpt-4.1-mini",
		},
	})
	if err != nil {
		t.Fatalf("buildLLMRuntime returned error: %v", err)
	}

	gotNames := make([]string, 0, len(providers))
	for _, provider := range providers {
		gotNames = append(gotNames, provider.Name())
	}
	wantNames := []string{"anthropic", "openai", "deepseek"}
	if strings.Join(gotNames, ",") != strings.Join(wantNames, ",") {
		t.Fatalf("provider order = %v, want %v", gotNames, wantNames)
	}
	if semanticModels["deepseek"] != "deepseek-chat" {
		t.Fatalf("semanticModels[deepseek] = %q", semanticModels["deepseek"])
	}
	if queryModels["openai"] != "gpt-4.1-mini" {
		t.Fatalf("queryModels[openai] = %q", queryModels["openai"])
	}
}

func TestBuildLLMRuntimeRejectsUnsupportedDefaultProvider(t *testing.T) {
	t.Parallel()

	_, _, _, err := buildLLMRuntime(config.Config{
		DefaultLLMProvider: "unsupported",
		ProviderAPIKeys:    map[string]string{"anthropic": "anthropic-key"},
	})
	if err == nil {
		t.Fatal("buildLLMRuntime returned nil error for unsupported default provider")
	}
}

func TestBuildLLMRuntimeRejectsUnconfiguredDefaultProvider(t *testing.T) {
	t.Parallel()

	_, _, _, err := buildLLMRuntime(config.Config{
		DefaultLLMProvider: "openai",
		ProviderAPIKeys:    map[string]string{"deepseek": "deepseek-key"},
	})
	if err == nil {
		t.Fatal("buildLLMRuntime returned nil error for unconfigured default provider")
	}
}

func TestBuildLLMRuntimeRequiresAtLeastOneProvider(t *testing.T) {
	t.Parallel()

	_, _, _, err := buildLLMRuntime(config.Config{
		DefaultLLMProvider: llmprovider.DefaultProviderName,
	})
	if err == nil {
		t.Fatal("buildLLMRuntime returned nil error without configured providers")
	}
}

func TestRegisterFrontendRoutesPreservesBackendRoutes(t *testing.T) {
	t.Parallel()

	frontend := newRouterTestFrontendHandler(t)
	router := chi.NewRouter()
	router.Get("/app-config.json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{}"))
	})
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
			name:   "app config",
			method: http.MethodGet,
			target: "/app-config.json",
			want:   "{}",
		},
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
			name:         "app config path is not shadowed",
			target:       "/app-config.json",
			wantContains: "",
			wantCode:     http.StatusNotFound,
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
