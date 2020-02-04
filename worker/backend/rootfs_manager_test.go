package backend_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/concourse/worker/backend"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RootfsManagerSuite struct {
	suite.Suite
	*require.Assertions

	rootfs              string
	baseSpec            *specs.Spec
	specWithUIDMappings *specs.Spec
}

func (s *RootfsManagerSuite) SetupTest() {
	var err error

	s.rootfs, err = ioutil.TempDir("", "rootfs-mgr")
	s.NoError(err)

	s.baseSpec = &specs.Spec{
		Root: &specs.Root{
			Path: s.rootfs,
		},
		Linux: &specs.Linux{},
	}
}

func (s *RootfsManagerSuite) TearDownTest() {
	os.RemoveAll(s.rootfs)
}

func (s *RootfsManagerSuite) TestSetupCwdDirAlreadyExists() {
	mkdirCalled := false
	mgr := backend.NewRootfsManager(
		backend.WithMkdirAll(func(p string, mode os.FileMode) error {
			mkdirCalled = true
			return nil
		}),
	)

	path := filepath.Join(s.rootfs, "dir")
	err := os.MkdirAll(path, 0755)
	s.NoError(err)

	err = mgr.SetupCwd(s.baseSpec, "dir")
	s.NoError(err)
	s.False(mkdirCalled, "does not call mkdir")
}

func (s *RootfsManagerSuite) TestSetupCwdCreatePathsRecursivelyByDefault() {
	mgr := backend.NewRootfsManager()

	err := mgr.SetupCwd(s.baseSpec, "/this/that")
	s.NoError(err)

	finfo, err := os.Stat(filepath.Join(s.rootfs, "this", "that"))
	s.NoError(err)
	s.True(finfo.IsDir())
}

func (s *RootfsManagerSuite) TestSetupCwdWithoutIDMappings() {
	var (
		path, expectedPath             = "", filepath.Join(s.rootfs, "dir")
		mode, expectedMode os.FileMode = 0000, 0777
	)

	mgr := backend.NewRootfsManager(
		backend.WithMkdirAll(func(p string, m os.FileMode) error {
			path = p
			mode = m
			return nil
		}),
	)

	err := mgr.SetupCwd(s.baseSpec, "dir")
	s.NoError(err)

	s.Equal(expectedPath, path)
	s.Equal(expectedMode, mode)
}
