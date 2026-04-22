package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bryanbaek/mission/internal/controlplane/reqlog"
)

type Router struct {
	defaultProvider string
	providers       map[string]Provider
}

func NewRouter(defaultProvider string, providers ...Provider) *Router {
	router := &Router{
		defaultProvider: strings.TrimSpace(defaultProvider),
		providers:       make(map[string]Provider, len(providers)),
	}
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		router.providers[provider.Name()] = provider
	}
	return router
}

func (r *Router) Name() string {
	return "router"
}

func (r *Router) Complete(
	ctx context.Context,
	req CompletionRequest,
) (CompletionResponse, error) {
	providerName := r.defaultProvider
	if providerName == "" {
		for name := range r.providers {
			providerName = name
			break
		}
	}
	provider, ok := r.providers[providerName]
	if !ok {
		return CompletionResponse{}, fmt.Errorf(
			"llm provider %q is not configured",
			providerName,
		)
	}
	start := time.Now()
	resp, err := provider.Complete(ctx, req)
	durationMS := time.Since(start).Milliseconds()

	if err != nil {
		return CompletionResponse{}, err
	}
	if resp.Provider == "" {
		resp.Provider = provider.Name()
	}
	reqlog.Logger(ctx).InfoContext(ctx, "llm.complete",
		"provider", resp.Provider,
		"model", resp.Model,
		"duration_ms", durationMS,
		"input_tokens", resp.Usage.InputTokens,
		"output_tokens", resp.Usage.OutputTokens,
		"cache_creation_tokens", resp.Usage.CacheCreationInputTokens,
		"cache_read_tokens", resp.Usage.CacheReadInputTokens,
	)
	return resp, nil
}
