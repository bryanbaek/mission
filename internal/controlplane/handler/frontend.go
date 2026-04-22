package handler

import (
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var reservedFrontendPrefixes = []string{
	"/app-config.json",
	"/healthz",
	"/api",
	"/tenant.v1.TenantService",
	"/semantic.v1.SemanticLayerService",
	"/query.v1.QueryService",
	"/starter.v1.StarterQuestionsService",
	"/onboarding.v1.OnboardingService",
	"/agent.v1.AgentService",
}

type FrontendHandler struct {
	root      string
	indexPath string
}

func NewFrontendHandler(root string) (*FrontendHandler, error) {
	indexPath := filepath.Join(root, "index.html")
	info, err := os.Stat(indexPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("frontend index.html is a directory")
	}
	return &FrontendHandler{
		root:      root,
		indexPath: indexPath,
	}, nil
}

func (h *FrontendHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if isReservedFrontendPath(r.URL.Path) {
		http.NotFound(w, r)
		return
	}

	if filePath, ok := h.resolveRequestedPath(r.URL.Path); ok {
		http.ServeFile(w, r, filePath)
		return
	}
	if path.Ext(path.Clean("/"+r.URL.Path)) != "" {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, h.indexPath)
}

func (h *FrontendHandler) resolveRequestedPath(requestPath string) (string, bool) {
	cleaned := path.Clean("/" + requestPath)
	if cleaned == "/" {
		return h.indexPath, true
	}

	relative := strings.TrimPrefix(cleaned, "/")
	candidate := filepath.Join(h.root, filepath.FromSlash(relative))
	info, err := os.Stat(candidate)
	switch {
	case err == nil && !info.IsDir():
		return candidate, true
	case err == nil && info.IsDir():
		indexCandidate := filepath.Join(candidate, "index.html")
		indexInfo, indexErr := os.Stat(indexCandidate)
		if indexErr == nil && !indexInfo.IsDir() {
			return indexCandidate, true
		}
	case !errors.Is(err, os.ErrNotExist):
		return "", false
	}

	return "", false
}

func isReservedFrontendPath(requestPath string) bool {
	for _, prefix := range reservedFrontendPrefixes {
		if requestPath == prefix || strings.HasPrefix(requestPath, prefix+"/") {
			return true
		}
	}
	return false
}
