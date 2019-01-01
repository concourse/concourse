package healthcheck

import (
	"context"
)

type Checker interface {
	// Check performs a health check with its execution time
	// limited by a context that might be cancelled at any
	// time.
	Check(ctx context.Context) (err error)
}
