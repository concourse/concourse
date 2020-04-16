package atccmd_test

import (
	"fmt"
	"testing"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/atccmd"
	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/acme/autocert"
)

type CommandSuite struct {
	suite.Suite
	*require.Assertions
}

func (s *CommandSuite) TestLetsEncryptDefaultIsUpToDate() {
	cmd := &atccmd.ATCCommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

	opt := parser.Find("run").FindOptionByLongName("lets-encrypt-acme-url")
	s.NotNil(opt)

	s.Equal(opt.Default, []string{autocert.DefaultACMEDirectory})
}

func (s *CommandSuite) TestInvalidConcurrentRequestLimitAction() {
	cmd := &atccmd.RunCommand{}
	parser := flags.NewParser(cmd, flags.None)
	_, err := parser.ParseArgs([]string{
		"--client-secret",
		"client-secret",
		"--concurrent-request-limit",
		fmt.Sprintf("%s:2", atc.GetInfo),
	})

	s.Contains(
		err.Error(),
		fmt.Sprintf("action '%s' is not supported", atc.GetInfo),
	)
}

func TestSuite(t *testing.T) {
	suite.Run(t, &CommandSuite{
		Assertions: require.New(t),
	})
}
