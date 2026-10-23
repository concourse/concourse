//go:build linux

package runtime

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/moby/sys/user"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	DefaultUid = 0
	DefaultGid = 0
)

// RootfsManagerOpt defines a functional option that when applied, modifies the
// configuration of a rootfsManager.
type RootfsManagerOpt func(m *rootfsManager)

// WithMkdirAll configures the function to be used for creating directories
// recursively.
func WithMkdirAll(f func(path string, mode os.FileMode) error) RootfsManagerOpt {
	return func(m *rootfsManager) {
		m.mkdirall = f
	}
}

type rootfsManager struct {
	mkdirall func(name string, mode os.FileMode) error
}

var _ RootfsManager = (*rootfsManager)(nil)

// NewRootfsManager instantiates a rootfsManager
func NewRootfsManager(opts ...RootfsManagerOpt) *rootfsManager {
	m := &rootfsManager{
		mkdirall: os.MkdirAll,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (r rootfsManager) SetupCwd(rootfsPath string, cwd string) error {
	abs := filepath.Join(rootfsPath, cwd)

	_, err := os.Stat(abs)
	if err == nil { // exists
		return nil
	}

	err = r.mkdirall(abs, 0777)
	if err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	return nil
}

// Returns the corresponding UID and GID for a given username by searching /etc/passwd and /etc/group.
//
// If username is a numeric user id or group id, it won't be evaluated but the metadata will be filled.
// e.g. username = "1001" will produce user with UID 1007.
// This is useful when an image doesn't have an /etc/passwd such as distroless images.
func (r rootfsManager) LookupUser(rootfsPath string, username string) (specs.User, bool, error) {
	passwdPath := filepath.Join(rootfsPath, "etc", "passwd")
	groupPath := filepath.Join(rootfsPath, "etc", "group")

	_, err := os.Stat(passwdPath)
	if os.IsNotExist(err) && username == "root" {
		return specs.User{UID: DefaultUid, GID: DefaultGid}, true, nil
	}

	execUser, err := user.GetExecUserPath(username, &user.ExecUser{Uid: DefaultUid, Gid: DefaultGid}, passwdPath, groupPath)
	if err != nil {
		return specs.User{}, false, err
	}

	return specs.User{UID: uint32(execUser.Uid), GID: uint32(execUser.Gid)}, true, nil
}
