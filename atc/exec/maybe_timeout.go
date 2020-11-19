package exec

import (
	"context"
	"fmt"
	"time"
)

func MaybeTimeout(ctx context.Context, timeoutStr string) (context.Context, func(), error) {
	if timeoutStr == "" {
		return ctx, func() {}, nil
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, nil, fmt.Errorf("parse timeout: %w", err)
	}

	processCtx, cancel := context.WithTimeout(ctx, timeout)
	return processCtx, cancel, nil
}
