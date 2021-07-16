package integration_test

import (
	"io/ioutil"
	"os/user"
	"sync"
	"testing"

	gouuid "github.com/nu7hatch/gouuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type buffer struct {
	content string
	sync.Mutex
}

func (m *buffer) Write(p []byte) (n int, err error) {
	m.Lock()
	m.content += string(p)
	m.Unlock()
	return len(p), nil
}

func (m *buffer) String() string {
	return m.content
}

func uuid() string {
	u4, err := gouuid.NewV4()
	if err != nil {
		panic("couldn't create new uuid")
	}

	return u4.String()
}

func TestSuite(t *testing.T) {
	req := require.New(t)

	user, err := user.Current()
	req.NoError(err)

	if user.Uid != "0" {
		t.Skip("must be run as root")
		return
	}

	tmpDir, err := ioutil.TempDir("", "containerd-test")
	if err != nil {
		panic(err)
	}
	suite.Run(t, &IntegrationSuite{
		Assertions: req,
		tmpDir:     tmpDir,
	})
}
