package atccmd_test

import (
	"fmt"
	"testing"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/atccmd"
	"github.com/concourse/concourse/flag/binder"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/acme/autocert"
)

type CommandSuite struct {
	suite.Suite
	*require.Assertions
}

func (s *CommandSuite) TestLetsEncryptDefaultIsUpToDate() {
	fs := pflag.NewFlagSet("run", pflag.ContinueOnError)
	b := binder.NewRegistry("CONCOURSE_").Binder(fs)
	s.NoError(b.Bind(&atccmd.RunCommand{}, ""))

	for _, info := range b.Flags() {
		if info.Name == "lets-encrypt-acme-url" {
			s.Equal([]string{autocert.DefaultACMEDirectory}, info.Defaults)
			return
		}
	}

	s.Fail("flag --lets-encrypt-acme-url not found")
}

func (s *CommandSuite) TestInvalidConcurrentRequestLimitAction() {
	fs := pflag.NewFlagSet("run", pflag.ContinueOnError)
	b := binder.NewRegistry("CONCOURSE_").Binder(fs)
	s.NoError(b.Bind(&atccmd.RunCommand{}, ""))

	err := fs.Parse([]string{
		"--client-secret",
		"client-secret",
		"--concurrent-request-limit",
		fmt.Sprintf("%s:2", atc.GetInfo),
	})

	s.Error(err)
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
