package runtime

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . RootfsManager

// RootfsManager is responsible for mutating the rootfs of a container.
//
type RootfsManager interface {
	// SetupCwd mutates the root filesystem to guarantee the presence of a
	// directory to be used as `cwd`.
	//
	SetupCwd(containerSpec *specs.Spec, cwd string) (err error)
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

func (r rootfsManager) SetupCwd(containerSpec *specs.Spec, cwd string) error {
	abs := filepath.Join(containerSpec.Root.Path, cwd)

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
