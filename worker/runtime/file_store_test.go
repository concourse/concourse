package runtime_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/concourse/worker/runtime"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type FileStoreSuite struct {
	suite.Suite
	*require.Assertions

	rootfs string
	store  runtime.FileStore
}

func (s *FileStoreSuite) SetupTest() {
	var err error

	s.rootfs, err = ioutil.TempDir("", "bcknd-filestore")
	s.NoError(err)

	s.store = runtime.NewFileStore(s.rootfs)
}

func (s *FileStoreSuite) TearDownTest() {
	os.RemoveAll(s.rootfs)
}

func (s *FileStoreSuite) TestCreateFile() {
	fpath, err := s.store.Create("name", []byte("hey"))
	s.NoError(err)

	content, err := ioutil.ReadFile(fpath)
	s.NoError(err)
	s.Equal("hey", string(content))
}

func (s *FileStoreSuite) TestCreateFileInDir() {
	fpath, err := s.store.Create("dir/name", []byte("hey"))
	s.NoError(err)

	content, err := ioutil.ReadFile(fpath)
	s.NoError(err)
	s.Equal("hey", string(content))
}

func (s *FileStoreSuite) TestDeleteFile() {
	fpath, err := s.store.Create("dir/name", []byte("hey"))
	s.NoError(err)

	err = s.store.Delete("dir/name")
	s.NoError(err)

	_, err = os.Stat(fpath)
	s.True(os.IsNotExist(err))
}

func (s *FileStoreSuite) TestDeleteDir() {
	fpath, err := s.store.Create("dir/name", []byte("hey"))
	s.NoError(err)

	err = s.store.Delete("dir")
	s.NoError(err)

	_, err = os.Stat(filepath.Dir(fpath))
	s.True(os.IsNotExist(err))
}
