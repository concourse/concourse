package runtime_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestSuite(t *testing.T) {
	suite.Run(t, &BackendSuite{Assertions: require.New(t)})
	suite.Run(t, &CNINetworkSuite{Assertions: require.New(t)})
	suite.Run(t, &ContainerSuite{Assertions: require.New(t)})
	suite.Run(t, &FileStoreSuite{Assertions: require.New(t)})
	suite.Run(t, &KillerSuite{Assertions: require.New(t)})
	suite.Run(t, &ProcessKillerSuite{Assertions: require.New(t)})
	suite.Run(t, &ProcessSuite{Assertions: require.New(t)})
	suite.Run(t, &RootfsManagerSuite{Assertions: require.New(t)})
	suite.Run(t, &UserNamespaceSuite{Assertions: require.New(t)})
	suite.Run(t, &TimeoutLockSuite{Assertions: require.New(t)})
}
