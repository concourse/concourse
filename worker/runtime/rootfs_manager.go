//go:build linux || windows

package runtime

import (
	"fmt"

	"github.com/opencontainers/runtime-spec/specs-go"
)

//counterfeiter:generate . RootfsManager

type InvalidUidError struct {
	UID string
}
type InvalidGidError struct {
	GID string
}

func (e InvalidUidError) Error() string {
	return fmt.Sprintf("invalid uid: %s", e.UID)
}

func (e InvalidGidError) Error() string {
	return fmt.Sprintf("invalid gid: %s", e.GID)
}

// RootfsManager is responsible for mutating and reading from the rootfs of a
// container.
type RootfsManager interface {
	// SetupCwd mutates the root filesystem to guarantee the presence of a
	// directory to be used as `cwd`.
	//
	SetupCwd(rootfsPath string, cwd string) (err error)

	// LookupUser resolves the specified username against the root
	// filesystem.
	//
	LookupUser(rootfsPath string, username string) (specs.User, bool, error)
}
