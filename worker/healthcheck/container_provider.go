package healthcheck

import (
	"context"
	"time"
)

//go:generate counterfeiter . ContainerProvider
type ContainerProvider interface {
	Create(ctx context.Context, handle, rootfs string, ttl time.Duration) (err error)
}
