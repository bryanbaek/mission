package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bryanbaek/mission/internal/controlplane/db"
	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	"github.com/bryanbaek/mission/internal/controlplane/llmprovider"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: preflight-helper migrations-status --database-url=<url>")
	}

	switch args[0] {
	case "migrations-status":
		return runMigrationsStatus(args[1:])
	case "llm-ping":
		return runLLMPing(args[1:])
	case "anthropic-ping":
		return runLegacyProviderPing("anthropic", args[1:])
	case "openai-ping":
		return runLegacyProviderPing("openai", args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runMigrationsStatus(args []string) error {
	fs := flag.NewFlagSet("migrations-status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var databaseURL string
	fs.StringVar(&databaseURL, "database-url", "", "Postgres database URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if databaseURL == "" {
		return fmt.Errorf("--database-url is required")
	}

	status, err := db.MigrationState(databaseURL)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(status)
}

func runLLMPing(args []string) error {
	fs := flag.NewFlagSet("llm-ping", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var provider string
	var apiKey string
	var model string
	var baseURL string
	fs.StringVar(&provider, "provider", "", "LLM provider name")
	fs.StringVar(&apiKey, "api-key", "", "LLM provider API key")
	fs.StringVar(&model, "model", "", "LLM provider model")
	fs.StringVar(&baseURL, "base-url", "", "Override provider base URL")
	if err := fs.Parse(args); err != nil {
		return err
	}

	return pingProvider(provider, apiKey, model, baseURL)
}

func runLegacyProviderPing(provider string, args []string) error {
	fs := flag.NewFlagSet(provider+"-ping", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var apiKey string
	var model string
	var baseURL string
	fs.StringVar(&apiKey, "api-key", "", "LLM provider API key")
	fs.StringVar(&model, "model", "", "LLM provider model")
	fs.StringVar(&baseURL, "base-url", "", "Override provider base URL")
	if err := fs.Parse(args); err != nil {
		return err
	}

	return pingProvider(provider, apiKey, model, baseURL)
}

func pingProvider(provider, apiKey, model, baseURL string) error {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return fmt.Errorf("--provider is required")
	}
	if _, ok := llmprovider.ByName(provider); !ok {
		return fmt.Errorf("unsupported provider %q", provider)
	}
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("--api-key is required")
	}
	if strings.TrimSpace(model) == "" {
		return fmt.Errorf("--model is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	providerClient, err := llmprovider.BuildWithBaseURL(
		provider,
		apiKey,
		baseURL,
		&http.Client{Timeout: 30 * time.Second},
	)
	if err != nil {
		return err
	}

	_, err = providerClient.Complete(ctx, llm.CompletionRequest{
		Operation: "preflight.anthropic_ping",
		Model:     model,
		MaxTokens: 1,
		Messages: []llm.Message{{
			Role:    "user",
			Content: "ping",
		}},
	})
	return err
}
