//go:build linux

package runtime

import (
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
