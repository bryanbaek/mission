package llm

import (
	"context"
	"fmt"
	"strings"
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
	resp, err := provider.Complete(ctx, req)
	if err != nil {
		return CompletionResponse{}, err
	}
	if resp.Provider == "" {
		resp.Provider = provider.Name()
	}
	return resp, nil
}
