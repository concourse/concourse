package worker

import (
	"context"
	"time"
)

type intContextKey int

// interruptTimeoutKey is the key for time.Duration values in Contexts. It is
// unexported. Clients should use WithInterruptTimeout & InterruptTimeoutFromContext
var interruptTimeoutKey intContextKey = 0

// WithInterruptTimeout returns a new Context that carries value t.
func WithInterruptTimeout(ctx context.Context, t time.Duration) context.Context {
	return context.WithValue(ctx, interruptTimeoutKey, t)
}

// InterruptTimeoutFromContext returns the time.Duration value stored in ctx, if any.
func InterruptTimeoutFromContext(ctx context.Context) (time.Duration, bool) {
	u, ok := ctx.Value(interruptTimeoutKey).(time.Duration)
	return u, ok
}
