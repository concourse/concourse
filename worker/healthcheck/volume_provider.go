package healthcheck

import (
	"context"
)

type Volume struct {
	Path   string `json:"path,omitempty"`
	Handle string `json:"handle,omitempty"`
}

//go:generate counterfeiter . VolumeProvider
type VolumeProvider interface {
	Create(ctx context.Context, handle string) (vol *Volume, err error)
	Destroy(ctx context.Context, handle string) (err error)
}
