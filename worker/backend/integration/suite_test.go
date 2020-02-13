package integration_test

import (
	"io/ioutil"
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
	tmpDir, err := ioutil.TempDir("", "containerd-test")
	if err != nil {
		panic(err)
	}
	suite.Run(t, &IntegrationSuite{
		Assertions: require.New(t),
		tmpDir:     tmpDir,
	})
}
