package handler

import (
	"encoding/json"
	"net/http"
)

type FrontendRuntimeConfig struct {
	ClerkPublishableKey string `json:"clerkPublishableKey,omitempty"`
	SentryDSN           string `json:"sentryDsn,omitempty"`
	SentryEnvironment   string `json:"sentryEnvironment,omitempty"`
	SentryRelease       string `json:"sentryRelease,omitempty"`
}

type FrontendRuntimeConfigHandler struct {
	config FrontendRuntimeConfig
}

func NewFrontendRuntimeConfigHandler(
	config FrontendRuntimeConfig,
) *FrontendRuntimeConfigHandler {
	return &FrontendRuntimeConfigHandler{config: config}
}

func (h *FrontendRuntimeConfigHandler) JavaScript(
	w http.ResponseWriter,
	_ *http.Request,
) {
	payload, err := json.Marshal(h.config)
	if err != nil {
		http.Error(w, "encode frontend config", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte("window.__MISSION_CONFIG__ = "))
	_, _ = w.Write(payload)
	_, _ = w.Write([]byte(";\n"))
}

func (h *FrontendRuntimeConfigHandler) JSON(
	w http.ResponseWriter,
	_ *http.Request,
) {
	payload, err := json.Marshal(h.config)
	if err != nil {
		http.Error(w, "encode frontend config", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(payload)
}
