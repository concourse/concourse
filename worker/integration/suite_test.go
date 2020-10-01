package integration_test

import (
	"os/user"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestSuite(t *testing.T) {
	req := require.New(t)

	user, err := user.Current()
	req.NoError(err)

	if user.Uid != "0" {
		t.Skip("must be run as root")
		return
	}

	suite.Run(t, &WorkerRunnerSuite{
		Assertions: req,
	})
}
