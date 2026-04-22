package llm

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("llm provider circuit breaker is open")

const (
	breakerFailureThreshold = 3
	breakerOpenInterval     = 30 * time.Second
)

type breakerState string

const (
	breakerStateClosed   breakerState = "closed"
	breakerStateOpen     breakerState = "open"
	breakerStateHalfOpen breakerState = "half_open"
)

type providerBreaker struct {
	mu               sync.Mutex
	provider         string
	now              func() time.Time
	state            breakerState
	consecutiveFails int
	openUntil        time.Time
	halfOpenInFlight bool
}

func newProviderBreaker(provider string) *providerBreaker {
	return &providerBreaker{
		provider: provider,
		now:      time.Now,
		state:    breakerStateClosed,
	}
}

func (b *providerBreaker) beforeCall() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	switch b.state {
	case breakerStateOpen:
		if now.Before(b.openUntil) {
			return NewTransientProviderError(b.provider, ErrCircuitOpen)
		}
		b.state = breakerStateHalfOpen
		b.halfOpenInFlight = false
		slog.Warn("llm circuit breaker half-open", "provider", b.provider)
	case breakerStateHalfOpen:
		if b.halfOpenInFlight {
			return NewTransientProviderError(b.provider, ErrCircuitOpen)
		}
	}

	if b.state == breakerStateHalfOpen {
		b.halfOpenInFlight = true
	}
	return nil
}

func (b *providerBreaker) afterCall(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	switch {
	case err == nil:
		wasHalfOpen := b.state == breakerStateHalfOpen
		b.state = breakerStateClosed
		b.consecutiveFails = 0
		b.openUntil = time.Time{}
		b.halfOpenInFlight = false
		if wasHalfOpen {
			slog.Info("llm circuit breaker closed", "provider", b.provider)
		}
	case errors.Is(err, context.Canceled):
		if b.state == breakerStateHalfOpen {
			b.halfOpenInFlight = false
		}
	case !IsTransientProviderError(err):
		wasHalfOpen := b.state == breakerStateHalfOpen
		b.state = breakerStateClosed
		b.consecutiveFails = 0
		b.openUntil = time.Time{}
		b.halfOpenInFlight = false
		if wasHalfOpen {
			slog.Info("llm circuit breaker closed", "provider", b.provider)
		}
	default:
		if b.state == breakerStateHalfOpen {
			b.tripLocked(now)
			return
		}
		b.consecutiveFails++
		if b.consecutiveFails >= breakerFailureThreshold {
			b.tripLocked(now)
		}
	}
}

func (b *providerBreaker) tripLocked(now time.Time) {
	wasOpen := b.state == breakerStateOpen
	b.state = breakerStateOpen
	b.consecutiveFails = 0
	b.openUntil = now.Add(breakerOpenInterval)
	b.halfOpenInFlight = false
	if !wasOpen {
		slog.Warn(
			"llm circuit breaker opened",
			"provider",
			b.provider,
			"retry_after",
			breakerOpenInterval.String(),
		)
	}
}
