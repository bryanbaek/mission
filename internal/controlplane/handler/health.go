package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type pinger interface {
	Ping(ctx context.Context) error
}

type HealthHandler struct {
	pool pinger
}

func NewHealthHandler(pool pinger) *HealthHandler {
	return &HealthHandler{pool: pool}
}

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{Status: "ok", Database: "ok"}
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.pool.Ping(ctx); err != nil {
		resp.Status = "degraded"
		resp.Database = "unreachable"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	_ = json.NewEncoder(w).Encode(resp)
}
