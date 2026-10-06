//go:build linux || windows

package runtime

import (
	"context"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// NoopNetwork leaves container networking entirely up to the container
// runtime's defaults. It is used on platforms where Concourse doesn't manage
// container networking itself (e.g. Windows).
type NoopNetwork struct{}

var _ Network = NoopNetwork{}

func NewNoopNetwork() NoopNetwork {
	return NoopNetwork{}
}

func (NoopNetwork) SetupHostNetwork() error {
	return nil
}

func (NoopNetwork) SetupMounts(handle string, hermetic bool) ([]specs.Mount, error) {
	return nil, nil
}

func (NoopNetwork) Add(ctx context.Context, task containerd.Task, containerHandle string) error {
	return nil
}

func (NoopNetwork) Remove(ctx context.Context, task containerd.Task, handle string) error {
	return nil
}

func (NoopNetwork) DropContainerTraffic(containerHandle string) error {
	return nil
}

func (NoopNetwork) ResumeContainerTraffic(containerHandle string) error {
	return nil
}
