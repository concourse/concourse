package spec

import (
	"fmt"
	"strings"
)

type PrivilegedMode int

const (
	// Full privileged mode means that privileged tasks can do everything.
	// It is equivalent to full root on the host.
	FullPrivilegedMode PrivilegedMode = iota
	// Ignore privileged mode means that privileged tasks have the same
	// powers as unprivileged tasks (effectively disabling privileged).
	IgnorePrivilegedMode
	// FUSE-only privileged mode means that privileged tasks have access to
	// create and mount FUSE filesystems. This is enough to use
	// fuse-overlayfs, but not to escape the container.
	FUSEOnlyPrivilegedMode
)

func (pm *PrivilegedMode) UnmarshalFlag(value string) error {
	switch strings.ToLower(value) {
	case "full":
		*pm = FullPrivilegedMode
	case "ignore":
		*pm = IgnorePrivilegedMode
	case "fuse-only":
		*pm = FUSEOnlyPrivilegedMode
	default:
		return fmt.Errorf("unsupported value for PrivilegedMode: %s", value)
	}
	return nil
}
