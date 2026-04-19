package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakePinger struct {
	err   error
	ctx   context.Context
	calls int
}

func (f *fakePinger) Ping(ctx context.Context) error {
	f.calls++
	f.ctx = ctx
	return f.err
}

func TestHealthzHealthy(t *testing.T) {
	t.Parallel()

	pinger := &fakePinger{}
	handler := NewHealthHandler(pinger)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.Healthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content type = %q, want application/json", rec.Header().Get("Content-Type"))
	}
	if pinger.calls != 1 {
		t.Fatalf("Ping called %d times, want 1", pinger.calls)
	}

	deadline, ok := pinger.ctx.Deadline()
	if !ok {
		t.Fatal("Ping context did not have deadline")
	}
	until := time.Until(deadline)
	if until < time.Second || until > 3*time.Second {
		t.Fatalf("deadline in %s, want roughly 2s", until)
	}

	var resp healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "ok" || resp.Database != "ok" {
		t.Fatalf("response = %+v, want ok/ok", resp)
	}
}

func TestHealthzDegraded(t *testing.T) {
	t.Parallel()

	handler := NewHealthHandler(&fakePinger{err: errors.New("database unavailable")})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.Healthz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content type = %q, want application/json", rec.Header().Get("Content-Type"))
	}

	var resp healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "degraded" || resp.Database != "unreachable" {
		t.Fatalf("response = %+v, want degraded/unreachable", resp)
	}
}
