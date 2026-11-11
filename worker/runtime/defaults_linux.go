//go:build linux

package runtime

import (
	"context"

	bespec "github.com/concourse/concourse/v8/worker/runtime/spec"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func defaultNetwork() (Network, error) {
	return NewCNINetwork()
}

func defaultSeccompProfiles() (profile, profileFuse specs.LinuxSeccomp) {
	return bespec.GetDefaultSeccompProfile(), bespec.GetDefaultSeccompProfileFuse()
}

func defaultTaskOpts() []containerd.NewTaskOpts {
	return []containerd.NewTaskOpts{containerd.WithNoNewKeyring}
}

func (b *GardenBackend) newContainer(ctx context.Context, handle string, labels map[string]string, oci *specs.Spec) (containerd.Container, error) {
	return b.client.NewContainer(ctx, handle, labels, oci)
}
