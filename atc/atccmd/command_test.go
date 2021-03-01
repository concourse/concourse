package atccmd_test

import (
	"testing"

	"github.com/concourse/concourse/atc/atccmd"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/acme/autocert"
)

type CommandSuite struct {
	suite.Suite
	*require.Assertions
}

func (s *CommandSuite) TestLetsEncryptDefaultIsUpToDate() {
	cmd := atccmd.CmdDefaults

	s.Equal(cmd.LetsEncrypt.ACMEURL.String(), autocert.DefaultACMEDirectory)
}

func TestSuite(t *testing.T) {
	suite.Run(t, &CommandSuite{
		Assertions: require.New(t),
	})
}
