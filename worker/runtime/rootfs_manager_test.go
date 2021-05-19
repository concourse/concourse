package runtime_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/concourse/worker/runtime"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RootfsManagerSuite struct {
	suite.Suite
	*require.Assertions

	rootfsPath string
}

func (s *RootfsManagerSuite) SetupTest() {
	var err error

	s.rootfsPath, err = ioutil.TempDir("", "rootfs-mgr")
	s.NoError(err)
}

func (s *RootfsManagerSuite) TearDownTest() {
	os.RemoveAll(s.rootfsPath)
}

func (s *RootfsManagerSuite) TestSetupCwdDirAlreadyExists() {
	mkdirCalled := false
	mgr := runtime.NewRootfsManager(
		runtime.WithMkdirAll(func(p string, mode os.FileMode) error {
			mkdirCalled = true
			return nil
		}),
	)

	path := filepath.Join(s.rootfsPath, "dir")
	err := os.MkdirAll(path, 0755)
	s.NoError(err)

	err = mgr.SetupCwd(s.rootfsPath, "dir")
	s.NoError(err)
	s.False(mkdirCalled, "does not call mkdir")
}

func (s *RootfsManagerSuite) TestSetupCwdCreatePathsRecursivelyByDefault() {
	mgr := runtime.NewRootfsManager()

	err := mgr.SetupCwd(s.rootfsPath, "/this/that")
	s.NoError(err)

	finfo, err := os.Stat(filepath.Join(s.rootfsPath, "this", "that"))
	s.NoError(err)
	s.True(finfo.IsDir())
}

func (s *RootfsManagerSuite) TestSetupCwdWithoutIDMappings() {
	var (
		path, expectedPath             = "", filepath.Join(s.rootfsPath, "dir")
		mode, expectedMode os.FileMode = 0000, 0777
	)

	mgr := runtime.NewRootfsManager(
		runtime.WithMkdirAll(func(p string, m os.FileMode) error {
			path = p
			mode = m
			return nil
		}),
	)

	err := mgr.SetupCwd(s.rootfsPath, "dir")
	s.NoError(err)

	s.Equal(expectedPath, path)
	s.Equal(expectedMode, mode)
}
