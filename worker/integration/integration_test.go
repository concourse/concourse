package integration_test

import (
	"crypto/rand"
	"crypto/rsa"
	"io/ioutil"
	"os"

	"github.com/concourse/concourse/worker/workercmd"
	"github.com/concourse/flag"
	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type WorkerRunnerSuite struct {
	suite.Suite
	*require.Assertions
	wrkcmd workercmd.WorkerCommand
}

func (s *WorkerRunnerSuite) BeforeTest(suiteName, testName string) {
	tmpdir, err := ioutil.TempDir("", suiteName+testName)
	s.NoError(err)

	err = os.Chdir(tmpdir)
	s.NoError(err)

	parser := flags.NewParser(&s.wrkcmd, flags.HelpFlag|flags.PassDoubleDash)
	parser.NamespaceDelimiter = "-"

	parser.FindOptionByLongName("baggageclaim-volumes").Required = false
	parser.FindOptionByLongName("tsa-worker-private-key").Required = false
	parser.FindOptionByLongName("work-dir").Required = false

	_, err = parser.ParseArgs([]string{""})
	s.NoError(err)

	signingKey, err := rsa.GenerateKey(rand.Reader, 2048)
	s.NoError(err)

	s.wrkcmd.TSA.WorkerPrivateKey = &flag.PrivateKey{PrivateKey: signingKey}
}

func (s *WorkerRunnerSuite) TestWorkDirIsCreated() {
	s.wrkcmd.WorkDir = "somedir"
	s.wrkcmd.Runtime = "containerd"

	_, err := s.wrkcmd.Runner([]string{})
	s.NoError(err)

	fileInfo, err := os.Stat("somedir")
	s.Equal(!os.IsNotExist(err), true)
	s.Equal(fileInfo.IsDir(), true)
	s.Equal(fileInfo.Mode().Perm(), os.FileMode(0755))
	s.NoError(err)
}
