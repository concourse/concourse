//go:build windows

package runtime

import (
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
