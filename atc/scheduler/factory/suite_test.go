package factory_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestSuite(t *testing.T) {
	suite.Run(t, &BuildFactorySuite{
		Assertions: require.New(t),
	})
}
