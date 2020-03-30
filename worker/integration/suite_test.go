package integration_test

import (
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestSuite(t *testing.T) {
	suite.Run(t, &WorkerRunnerSuite{
		Assertions: require.New(t),
	})
}
