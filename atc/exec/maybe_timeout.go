package exec

import (
	"context"
	"fmt"
	"time"
)

func MaybeTimeout(ctx context.Context, timeoutStr string, defaultTimeout time.Duration, maxTimeout time.Duration) (context.Context, func(), error) {
	if timeoutStr == "" && defaultTimeout == 0 && maxTimeout == 0 {
		return ctx, func() {}, nil
	}

	timeout := defaultTimeout
	if timeoutStr != "" {
		var err error
		timeout, err = time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, nil, fmt.Errorf("parse timeout: %w", err)
		}
	}

	if maxTimeout > 0 && (timeout == 0 || timeout > maxTimeout) {
		timeout = maxTimeout
	}

	if timeout == 0 {
		return ctx, func() {}, nil
	}

	processCtx, cancel := context.WithTimeout(ctx, timeout)
	return processCtx, cancel, nil
}
