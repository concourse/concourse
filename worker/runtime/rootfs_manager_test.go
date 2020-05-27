package runtime_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/concourse/worker/runtime"
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
	mgr := runtime.NewRootfsManager(
		runtime.WithMkdirAll(func(p string, mode os.FileMode) error {
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
	mgr := runtime.NewRootfsManager()

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

	mgr := runtime.NewRootfsManager(
		runtime.WithMkdirAll(func(p string, m os.FileMode) error {
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

func (s *RootfsManagerSuite) TestLookupUserReturnsUserInfo() {
	mgr := runtime.NewRootfsManager()
	s.writeEtcPasswd(`
		root:*:0:0:System Administrator:/var/root:/bin/sh
		some_user_name:*:1:1:Some User:/var/root:/usr/bin/false
	`)
	actualUser, ok, err := mgr.LookupUser(s.baseSpec, "some_user_name")
	s.NoError(err)

	s.True(ok)
	expectedUser := specs.User{
		UID: 1,
		GID: 1,
	}
	s.Equal(expectedUser, actualUser)
}

func (s *RootfsManagerSuite) TestLookupUserUsernameNotFound() {
	mgr := runtime.NewRootfsManager()

	s.writeEtcPasswd(`
		root:*:0:0:System Administrator:/var/root:/bin/sh
		some_user_name:*:1:1:Some User:/var/root:/usr/bin/false
	`)

	_, ok, err := mgr.LookupUser(s.baseSpec, "missing_username")
	s.NoError(err)
	s.False(ok)
}

func (s *RootfsManagerSuite) TestLookupUserInvalidUID() {
	mgr := runtime.NewRootfsManager()

	s.writeEtcPasswd(`
		some_user_name:*:NaN:0:System Administrator:/var/root:/bin/sh
	`)

	_, _, err := mgr.LookupUser(s.baseSpec, "some_user_name")
	s.Error(err)
}

func (s *RootfsManagerSuite) TestLookupUserInvalidGID() {
	mgr := runtime.NewRootfsManager()

	s.writeEtcPasswd(`
		some_user_name:*:0:NaN:System Administrator:/var/root:/bin/sh
	`)

	_, _, err := mgr.LookupUser(s.baseSpec, "some_user_name")
	s.Error(err)
}

func (s *RootfsManagerSuite) TestLookupUserEtcPasswdNotFound() {
	mgr := runtime.NewRootfsManager()

	_, _, err := mgr.LookupUser(s.baseSpec, "username")
	s.Error(err)
}

func (s *RootfsManagerSuite) TestLookupUserIgnoreNonUserInfo() {
	mgr := runtime.NewRootfsManager()
	s.writeEtcPasswd(`
		#This is etc passwd
		root:*:0:0:System Administrator:/var/root:/bin/sh
		some_user_name:*:1

		some_user_name:*:1:1:Some User:/var/root:/usr/bin/false
	`)
	_, ok, err := mgr.LookupUser(s.baseSpec, "some_user_name")
	s.NoError(err)
	s.True(ok)
}

func (s *RootfsManagerSuite) writeEtcPasswd(contents string) {
	err := os.MkdirAll(filepath.Join(s.rootfs, "etc"), 0755)
	s.NoError(err)

	err = ioutil.WriteFile(filepath.Join(s.rootfs, "etc", "passwd"), []byte(contents), 0755)
	s.NoError(err)
}