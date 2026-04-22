package config

import (
	"testing"
	"time"

	"github.com/bryanbaek/mission/internal/controlplane/llmprovider"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mission:mission@localhost:5432/mission")
	clearProviderEnv(t)
	t.Setenv("ENV", "")
	t.Setenv("PORT", "")
	t.Setenv("HTTP_PORT", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("EDGE_AGENT_IMAGE", "")
	t.Setenv("EDGE_AGENT_VERSION", "")
	t.Setenv("EDGE_AGENT_IMAGE_REPOSITORY", "")
	t.Setenv("PUBLIC_CONTROL_PLANE_URL", "")
	t.Setenv("SHUTDOWN_TIMEOUT_SECONDS", "")
	t.Setenv("DB_MAX_CONNS", "")
	t.Setenv("DB_MIN_CONNS", "")
	t.Setenv("DB_HEALTH_CHECK_PERIOD_SECONDS", "")
	t.Setenv("VITE_SENTRY_DSN", "")
	t.Setenv("VITE_SENTRY_ENVIRONMENT", "")
	t.Setenv("VITE_SENTRY_RELEASE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Env != "development" {
		t.Fatalf("Env = %q, want development", cfg.Env)
	}
	if cfg.HTTPPort != 8080 {
		t.Fatalf("HTTPPort = %d, want 8080", cfg.HTTPPort)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.EdgeAgentVersion != "v0.1.0" {
		t.Fatalf("EdgeAgentVersion = %q, want v0.1.0", cfg.EdgeAgentVersion)
	}
	if cfg.EdgeAgentImageRepo != "registry.digitalocean.com/mission/edge-agent" {
		t.Fatalf("EdgeAgentImageRepo = %q", cfg.EdgeAgentImageRepo)
	}
	if cfg.EdgeAgentImage != "registry.digitalocean.com/mission/edge-agent:v0.1.0" {
		t.Fatalf("EdgeAgentImage = %q, want registry.digitalocean.com/mission/edge-agent:v0.1.0", cfg.EdgeAgentImage)
	}
	if cfg.PublicControlPlaneURL != "http://localhost:8080" {
		t.Fatalf("PublicControlPlaneURL = %q, want http://localhost:8080", cfg.PublicControlPlaneURL)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want 10s", cfg.ShutdownTimeout)
	}
	if cfg.DBMaxConns != 4 {
		t.Fatalf("DBMaxConns = %d, want 4", cfg.DBMaxConns)
	}
	if cfg.DBMinConns != 1 {
		t.Fatalf("DBMinConns = %d, want 1", cfg.DBMinConns)
	}
	if cfg.DBHealthCheckPeriod != 30*time.Second {
		t.Fatalf(
			"DBHealthCheckPeriod = %s, want 30s",
			cfg.DBHealthCheckPeriod,
		)
	}
	if cfg.FrontendSentryEnvironment != "development" {
		t.Fatalf("FrontendSentryEnvironment = %q, want development", cfg.FrontendSentryEnvironment)
	}
	if cfg.DefaultLLMProvider != llmprovider.DefaultProviderName {
		t.Fatalf("DefaultLLMProvider = %q, want %q", cfg.DefaultLLMProvider, llmprovider.DefaultProviderName)
	}
	if len(cfg.ProviderAPIKeys) != 0 {
		t.Fatalf("ProviderAPIKeys = %#v, want empty", cfg.ProviderAPIKeys)
	}
	if len(cfg.SemanticLayerProviderModels) != 0 {
		t.Fatalf("SemanticLayerProviderModels = %#v, want empty", cfg.SemanticLayerProviderModels)
	}
	if len(cfg.QueryProviderModels) != 0 {
		t.Fatalf("QueryProviderModels = %#v, want empty", cfg.QueryProviderModels)
	}
}

func TestLoadInvalidHTTPPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mission:mission@localhost:5432/mission")
	t.Setenv("PORT", "")
	t.Setenv("HTTP_PORT", "bad-port")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error for invalid HTTP_PORT")
	}
}

func TestLoadUsesCloudRunPortWhenPresent(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mission:mission@localhost:5432/mission")
	t.Setenv("PORT", "9090")
	t.Setenv("HTTP_PORT", "8081")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HTTPPort != 9090 {
		t.Fatalf("HTTPPort = %d, want 9090", cfg.HTTPPort)
	}
}

func TestLoadUsesCloudRunPortWhenOnlyPortSet(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mission:mission@localhost:5432/mission")
	t.Setenv("PORT", "7070")
	t.Setenv("HTTP_PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HTTPPort != 7070 {
		t.Fatalf("HTTPPort = %d, want 7070", cfg.HTTPPort)
	}
}

func TestLoadRejectsInvalidCloudRunPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mission:mission@localhost:5432/mission")
	t.Setenv("PORT", "bad-port")
	t.Setenv("HTTP_PORT", "8080")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error for invalid PORT")
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("HTTP_PORT", "8080")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error when DATABASE_URL is missing")
	}
}

func TestLoadUsesEdgeAgentOverride(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mission:mission@localhost:5432/mission")
	t.Setenv(
		"EDGE_AGENT_IMAGE",
		"registry.digitalocean.com/custom/edge-agent:release-7",
	)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.EdgeAgentImage != "registry.digitalocean.com/custom/edge-agent:release-7" {
		t.Fatalf("EdgeAgentImage = %q", cfg.EdgeAgentImage)
	}
}

func TestLoadUsesFrontendSentryOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mission:mission@localhost:5432/mission")
	t.Setenv("VITE_SENTRY_DSN", "https://public@example.com/1")
	t.Setenv("VITE_SENTRY_ENVIRONMENT", "preview")
	t.Setenv("VITE_SENTRY_RELEASE", "frontend-release")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.FrontendSentryDSN != "https://public@example.com/1" {
		t.Fatalf("FrontendSentryDSN = %q", cfg.FrontendSentryDSN)
	}
	if cfg.FrontendSentryEnvironment != "preview" {
		t.Fatalf("FrontendSentryEnvironment = %q", cfg.FrontendSentryEnvironment)
	}
	if cfg.FrontendSentryRelease != "frontend-release" {
		t.Fatalf("FrontendSentryRelease = %q", cfg.FrontendSentryRelease)
	}
}

func TestLoadRequiresPublicControlPlaneURLInProduction(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mission:mission@localhost:5432/mission")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_SECRET_KEY", "sk_live_example")
	t.Setenv("PUBLIC_CONTROL_PLANE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error without PUBLIC_CONTROL_PLANE_URL in production")
	}
}

func TestLoadUsesProviderSpecificModels(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mission:mission@localhost:5432/mission")
	clearProviderEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-key")
	t.Setenv("DEEPSEEK_API_KEY", "deepseek-key")
	t.Setenv("ANTHROPIC_SEMANTIC_LAYER_MODEL", "claude-sonnet-4-6")
	t.Setenv("DEEPSEEK_SEMANTIC_LAYER_MODEL", "deepseek-chat")
	t.Setenv("ANTHROPIC_QUERY_MODEL", "claude-haiku-4")
	t.Setenv("DEEPSEEK_QUERY_MODEL", "deepseek-chat")
	t.Setenv("DEEPSEEK_PREFLIGHT_MODEL", "deepseek-chat")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ProviderAPIKeys["anthropic"] != "anthropic-key" {
		t.Fatalf("ProviderAPIKeys[anthropic] = %q", cfg.ProviderAPIKeys["anthropic"])
	}
	if cfg.ProviderAPIKeys["deepseek"] != "deepseek-key" {
		t.Fatalf("ProviderAPIKeys[deepseek] = %q", cfg.ProviderAPIKeys["deepseek"])
	}
	if cfg.SemanticLayerProviderModels["anthropic"] != "claude-sonnet-4-6" {
		t.Fatalf(
			"SemanticLayerProviderModels[anthropic] = %q",
			cfg.SemanticLayerProviderModels["anthropic"],
		)
	}
	if cfg.SemanticLayerProviderModels["deepseek"] != "deepseek-chat" {
		t.Fatalf(
			"SemanticLayerProviderModels[deepseek] = %q",
			cfg.SemanticLayerProviderModels["deepseek"],
		)
	}
	if cfg.QueryProviderModels["anthropic"] != "claude-haiku-4" {
		t.Fatalf("QueryProviderModels[anthropic] = %q", cfg.QueryProviderModels["anthropic"])
	}
	if cfg.QueryProviderModels["deepseek"] != "deepseek-chat" {
		t.Fatalf("QueryProviderModels[deepseek] = %q", cfg.QueryProviderModels["deepseek"])
	}
	if cfg.PreflightProviderModels["deepseek"] != "deepseek-chat" {
		t.Fatalf("PreflightProviderModels[deepseek] = %q", cfg.PreflightProviderModels["deepseek"])
	}
}

func TestLoadRejectsInvalidDBPoolConfig(t *testing.T) {
	testCases := []struct {
		name    string
		max     string
		min     string
		health  string
		wantErr string
	}{
		{
			name:    "max must be positive",
			max:     "0",
			min:     "0",
			health:  "30",
			wantErr: "DB_MAX_CONNS must be greater than 0",
		},
		{
			name:    "min cannot be negative",
			max:     "4",
			min:     "-1",
			health:  "30",
			wantErr: "DB_MIN_CONNS must be greater than or equal to 0",
		},
		{
			name:    "min cannot exceed max",
			max:     "2",
			min:     "3",
			health:  "30",
			wantErr: "DB_MIN_CONNS must be less than or equal to DB_MAX_CONNS",
		},
		{
			name:    "health check must be positive",
			max:     "4",
			min:     "1",
			health:  "0",
			wantErr: "DB_HEALTH_CHECK_PERIOD_SECONDS must be greater than 0",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://mission:mission@localhost:5432/mission")
			t.Setenv("DB_MAX_CONNS", tc.max)
			t.Setenv("DB_MIN_CONNS", tc.min)
			t.Setenv("DB_HEALTH_CHECK_PERIOD_SECONDS", tc.health)

			_, err := Load()
			if err == nil {
				t.Fatal("Load returned nil error")
			}
			if err.Error() != tc.wantErr {
				t.Fatalf("err = %q, want %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func clearProviderEnv(t *testing.T) {
	t.Helper()

	for _, spec := range llmprovider.Specs() {
		t.Setenv(spec.APIKeyEnv, "")
		t.Setenv(spec.SemanticLayerModelEnv, "")
		t.Setenv(spec.QueryModelEnv, "")
		t.Setenv(spec.PreflightModelEnv, "")
	}
}

func TestGetenvUsesFallback(t *testing.T) {
	t.Setenv("EXAMPLE_KEY", "")
	if got := getenv("EXAMPLE_KEY", "fallback"); got != "fallback" {
		t.Fatalf("getenv fallback = %q, want fallback", got)
	}

	t.Setenv("EXAMPLE_KEY", "configured")
	if got := getenv("EXAMPLE_KEY", "fallback"); got != "configured" {
		t.Fatalf("getenv configured = %q, want configured", got)
	}
}

func TestResolveEdgeAgentImage(t *testing.T) {
	tests := []struct {
		name       string
		override   string
		repository string
		version    string
		want       string
	}{
		{
			name:       "override wins",
			override:   "registry.digitalocean.com/mission/edge-agent:manual",
			repository: "ignored",
			version:    "ignored",
			want:       "registry.digitalocean.com/mission/edge-agent:manual",
		},
		{
			name:       "compose repository and version",
			repository: "registry.digitalocean.com/mission/edge-agent",
			version:    "v0.1.0",
			want:       "registry.digitalocean.com/mission/edge-agent:v0.1.0",
		},
		{
			name:       "repository without version",
			repository: "registry.digitalocean.com/mission/edge-agent",
			want:       "registry.digitalocean.com/mission/edge-agent",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveEdgeAgentImage(tc.override, tc.repository, tc.version); got != tc.want {
				t.Fatalf("resolveEdgeAgentImage = %q, want %q", got, tc.want)
			}
		})
	}
}
