package healthcheck

import (
	"context"
)

//go:generate counterfeiter . ContainerProvider
type ContainerProvider interface {
	Create(ctx context.Context, handle, rootfs string) (err error)
	Destroy(ctx context.Context, handle string) (err error)
}
