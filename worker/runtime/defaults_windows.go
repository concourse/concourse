//go:build windows

package runtime

import (
	"context"
	"errors"
	"fmt"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func defaultNetwork() (Network, error) {
	return NewNoopNetwork(), nil
}

// defaultSeccompProfiles returns empty profiles; seccomp is a Linux-only
// concept and is never applied to Windows containers.
func defaultSeccompProfiles() (profile, profileFuse specs.LinuxSeccomp) {
	return specs.LinuxSeccomp{}, specs.LinuxSeccomp{}
}

func defaultTaskOpts() []containerd.NewTaskOpts {
	return nil
}

// newContainer creates the container from a snapshot of the image in its
// rootfs directory, an OCI image layout. The windows snapshotter applies each
// layer with the NTFS security descriptors and file attributes from its tar
// headers and with its own registry hive deltas.
func (b *GardenBackend) newContainer(ctx context.Context, handle string, labels map[string]string, oci *specs.Spec) (containerd.Container, error) {
	imageLayout := oci.Root.Path
	oci.Root = nil

	attacher, attach := b.network.(ContainerAttacher)
	if attach {
		err := attacher.AttachContainer(handle, oci)
		if err != nil {
			return nil, fmt.Errorf("attach container network: %w", err)
		}
	}

	cont, err := b.client.NewContainerFromImageLayout(ctx, handle, labels, oci, imageLayout)
	if err != nil && attach {
		return nil, errors.Join(err, attacher.DetachContainer(oci))
	}

	return cont, err
}
