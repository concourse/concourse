package runtime

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . RootfsManager

type InvalidUidError struct{
	UID string
}
type InvalidGidError struct{
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
//
type RootfsManager interface {
	// SetupCwd mutates the root filesystem to guarantee the presence of a
	// directory to be used as `cwd`.
	//
	SetupCwd(rootfsPath string, cwd string) (err error)

	// LookupUser scans the /etc/passwd file from the root filesystem for the
	// UID and GID of the specified username.
	//
	LookupUser(rootfsPath string, username string) (specs.User, bool, error)
}

// RootfsManagerOpt defines a functional option that when applied, modifies the
// configuration of a rootfsManager.
//
type RootfsManagerOpt func(m *rootfsManager)

// WithMkdirAll configures the function to be used for creating directories
// recursively.
//
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
//
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

// Mostly copied from Go's `os/user` package
// https://github.com/golang/go/blob/f296b7a6f045325a230f77e9bda1470b1270f817/src/os/user/lookup_unix.go#L35
func (r rootfsManager) LookupUser(rootfsPath string, username string) (specs.User, bool, error) {
	path := filepath.Join(rootfsPath, "etc", "passwd")
	file, err := os.Open(path)
	if err != nil {
		return specs.User{}, false, err
	}
	defer file.Close()
	bs := bufio.NewScanner(file)
	for bs.Scan() {
		line := bs.Text()

		// There's no spec for /etc/passwd or /etc/group, but we try to follow
		// the same rules as the glibc parser, which allows comments and blank
		// space at the beginning of a line.
		line = strings.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) != 7 {
			continue
		}
		if parts[0] != username {
			continue
		}
		var (
			uid int
			gid int
		)
		if uid, err = strconv.Atoi(parts[2]); err != nil {
			return specs.User{}, false, InvalidUidError{UID: parts[2]}
		}
		if gid, err = strconv.Atoi(parts[3]); err != nil {
			return specs.User{}, false, InvalidGidError{GID: parts[3]}
		}
		return specs.User{UID: uint32(uid), GID: uint32(gid)}, true, nil
	}
	return specs.User{}, false, bs.Err()
}
