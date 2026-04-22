package reqlog

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

// WithLogger stores a derived logger in ctx so downstream code can emit
// pre-seeded attributes (e.g. request_id) without re-extracting them.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// Logger returns the logger stored by WithLogger, or slog.Default() if none.
func Logger(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
