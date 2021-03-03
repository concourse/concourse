package spec

import (
	"os"

	"github.com/opencontainers/runtime-spec/specs-go"
)

var (
	PrivilegedContainerNamespaces = []specs.LinuxNamespace{
		{Type: specs.PIDNamespace},
		{Type: specs.IPCNamespace},
		{Type: specs.UTSNamespace},
		{Type: specs.MountNamespace},
		{Type: specs.NetworkNamespace},
	}

	UnprivilegedContainerNamespaces = append(PrivilegedContainerNamespaces,
		specs.LinuxNamespace{Type: specs.UserNamespace},
	)
)

func init() {
	if cgroupNamespacesSupported() {
		UnprivilegedContainerNamespaces = append(UnprivilegedContainerNamespaces,
			specs.LinuxNamespace{Type: specs.CgroupNamespace})
	}
}

func OciNamespaces(privileged bool) []specs.LinuxNamespace {
	if !privileged {
		return UnprivilegedContainerNamespaces
	}

	return PrivilegedContainerNamespaces
}

func cgroupNamespacesSupported() bool {
	_, err := os.Stat("/proc/self/ns/cgroup")
	return !os.IsNotExist(err)
}
