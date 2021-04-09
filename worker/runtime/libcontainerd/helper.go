package libcontainerd

import (
	"context"
	"time"
)

func createTimeoutContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout == 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}
