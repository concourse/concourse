package healthcheck

import (
	"context"
	"time"
)

type Volume struct {
	Path   string        `json:"path,omitempty"`
	Handle string        `json:"handle,omitempty"`
	TTL    time.Duration `json:"ttl,omitempty"`
}

//go:generate counterfeiter . VolumeProvider
type VolumeProvider interface {
	Create(ctx context.Context, handle string, ttl time.Duration) (vol *Volume, err error)
}
