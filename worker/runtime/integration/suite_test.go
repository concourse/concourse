package integration_test

import (
	"errors"
	"io/ioutil"
	"net"
	"os/user"
	"regexp"
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

func getHostIp() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	ethInterface := regexp.MustCompile("eth0")

	for _, i := range ifaces {
		if ethInterface.MatchString(i.Name) {
			addrs, err := i.Addrs()
			if err != nil {
				return "", err
			}
			for _, address := range addrs {
				if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						return ipnet.IP.String(), nil
					}
				}
			}
		}
	}
	return "", errors.New("unable to find host's IP")
}
