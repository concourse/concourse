//go:build windows

package runtime

import (
	"github.com/opencontainers/runtime-spec/specs-go"
)

// rootfsManager is mostly a no-op on Windows: the working directory is
// created by the runtime when the process starts, and users are resolved by
// name inside the container rather than from /etc/passwd.
type rootfsManager struct{}

var _ RootfsManager = rootfsManager{}

// NewRootfsManager instantiates a rootfsManager
func NewRootfsManager() rootfsManager {
	return rootfsManager{}
}

func (rootfsManager) SetupCwd(rootfsPath string, cwd string) error {
	return nil
}

func (rootfsManager) LookupUser(rootfsPath string, username string) (specs.User, bool, error) {
	return specs.User{Username: username}, true, nil
}
