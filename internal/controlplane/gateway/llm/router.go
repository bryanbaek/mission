package llm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bryanbaek/mission/internal/controlplane/reqlog"
)

type Router struct {
	defaultProvider string
	order           []string
	providers       map[string]Provider
	breakers        map[string]*providerBreaker
}

func NewRouter(defaultProvider string, providers ...Provider) *Router {
	router := &Router{
		defaultProvider: strings.TrimSpace(defaultProvider),
		providers:       make(map[string]Provider, len(providers)),
		breakers:        make(map[string]*providerBreaker, len(providers)),
	}
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		name := strings.TrimSpace(provider.Name())
		if name == "" {
			continue
		}
		if _, exists := router.providers[name]; exists {
			continue
		}
		router.order = append(router.order, name)
		router.providers[name] = provider
		router.breakers[name] = newProviderBreaker(name)
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
	candidates := r.orderedProviders()
	if len(candidates) == 0 {
		return CompletionResponse{}, fmt.Errorf("no llm providers are configured")
	}

	logger := reqlog.Logger(ctx)
	operation := strings.TrimSpace(req.Operation)
	attemptedProviders := make([]string, 0, len(candidates))
	var lastErr error

	for index, providerName := range candidates {
		provider := r.providers[providerName]
		breaker := r.breakers[providerName]
		if err := breaker.beforeCall(); err != nil {
			attemptedProviders = append(attemptedProviders, providerName)
			lastErr = err
			r.logFailover(ctx, logger, operation, providerName, candidates, index, err)
			continue
		}

		providerReq := req
		providerReq.Model = req.ModelForProvider(providerName)
		if providerReq.Model == "" {
			err := NewProviderError(
				providerName,
				fmt.Errorf("model is not configured for provider %q", providerName),
			)
			breaker.afterCall(err)
			return CompletionResponse{}, err
		}

		start := time.Now()
		resp, err := provider.Complete(ctx, providerReq)
		durationMS := time.Since(start).Milliseconds()
		err = WrapProviderError(providerName, err)
		breaker.afterCall(err)
		if err != nil {
			if !IsTransientProviderError(err) {
				return CompletionResponse{}, err
			}
			attemptedProviders = append(attemptedProviders, providerName)
			lastErr = err
			logger.WarnContext(ctx, "llm.complete.failed",
				"operation", operation,
				"provider", providerName,
				"duration_ms", durationMS,
				"err", err,
			)
			r.logFailover(ctx, logger, operation, providerName, candidates, index, err)
			continue
		}
		if resp.Provider == "" {
			resp.Provider = provider.Name()
		}
		logger.InfoContext(ctx, "llm.complete",
			"operation", operation,
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

	if lastErr != nil {
		return CompletionResponse{}, NewUnavailableError(attemptedProviders, lastErr)
	}
	return CompletionResponse{}, fmt.Errorf("no llm providers are configured")
}

func (r *Router) orderedProviders() []string {
	if len(r.order) == 0 {
		return nil
	}

	out := make([]string, 0, len(r.order))
	seen := make(map[string]struct{}, len(r.order))
	if preferred := strings.TrimSpace(r.defaultProvider); preferred != "" {
		if _, ok := r.providers[preferred]; ok {
			out = append(out, preferred)
			seen[preferred] = struct{}{}
		}
	}
	for _, name := range r.order {
		if _, ok := seen[name]; ok {
			continue
		}
		out = append(out, name)
	}
	return out
}

func (r *Router) logFailover(
	ctx context.Context,
	logger *slog.Logger,
	operation string,
	providerName string,
	candidates []string,
	index int,
	err error,
) {
	if index >= len(candidates)-1 {
		return
	}
	logger.WarnContext(ctx, "llm.failover",
		"operation", operation,
		"provider", providerName,
		"next_provider", candidates[index+1],
		"err", err,
	)
}
