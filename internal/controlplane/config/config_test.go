package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mission:mission@localhost:5432/mission")
	t.Setenv("ENV", "")
	t.Setenv("HTTP_PORT", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("EDGE_AGENT_IMAGE", "")
	t.Setenv("EDGE_AGENT_VERSION", "")
	t.Setenv("EDGE_AGENT_IMAGE_REPOSITORY", "")
	t.Setenv("PUBLIC_CONTROL_PLANE_URL", "")

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
}

func TestLoadInvalidHTTPPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mission:mission@localhost:5432/mission")
	t.Setenv("HTTP_PORT", "bad-port")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error for invalid HTTP_PORT")
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
