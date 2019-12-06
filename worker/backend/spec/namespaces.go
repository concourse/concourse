package spec

import "github.com/opencontainers/runtime-spec/specs-go"

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

func OciNamespaces(privileged bool) []specs.LinuxNamespace {
	if !privileged {
		return UnprivilegedContainerNamespaces
	}

	return PrivilegedContainerNamespaces
}
