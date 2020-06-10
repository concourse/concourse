package runtime_test

import (
	"errors"
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

func (s *RootfsManagerSuite) TestLookupUserReturnsUserInfo() {
	mgr := runtime.NewRootfsManager()
	s.writeEtcPasswd(`
		root:*:0:0:System Administrator:/var/root:/bin/sh
		some_user_name:*:1:1:Some User:/var/root:/usr/bin/false
	`)
	actualUser, ok, err := mgr.LookupUser(s.rootfsPath, "some_user_name")
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

	_, ok, err := mgr.LookupUser(s.rootfsPath, "missing_username")
	s.NoError(err)
	s.False(ok)
}

func (s *RootfsManagerSuite) TestLookupUserInvalidUID() {
	mgr := runtime.NewRootfsManager()

	s.writeEtcPasswd(`
		some_user_name:*:NaN:0:System Administrator:/var/root:/bin/sh
	`)

	_, _, err := mgr.LookupUser(s.rootfsPath, "some_user_name")
	s.True(errors.Is(err, runtime.InvalidUidError{UID: "NaN"}))
}

func (s *RootfsManagerSuite) TestLookupUserInvalidGID() {
	mgr := runtime.NewRootfsManager()

	s.writeEtcPasswd(`
		some_user_name:*:0:NaN:System Administrator:/var/root:/bin/sh
	`)

	_, _, err := mgr.LookupUser(s.rootfsPath, "some_user_name")
	s.True(errors.Is(err, runtime.InvalidGidError{GID: "NaN"}))
}

func (s *RootfsManagerSuite) TestLookupUserEtcPasswdNotFound() {
	mgr := runtime.NewRootfsManager()

	_, _, err := mgr.LookupUser(s.rootfsPath, "username")
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
	_, ok, err := mgr.LookupUser(s.rootfsPath, "some_user_name")
	s.NoError(err)
	s.True(ok)
}

func (s *RootfsManagerSuite) writeEtcPasswd(contents string) {
	err := os.MkdirAll(filepath.Join(s.rootfsPath, "etc"), 0755)
	s.NoError(err)

	err = ioutil.WriteFile(filepath.Join(s.rootfsPath, "etc", "passwd"), []byte(contents), 0755)
	s.NoError(err)
}