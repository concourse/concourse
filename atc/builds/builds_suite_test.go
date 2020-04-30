package builds_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"testing"
)

func TestBuilds(t *testing.T) {
	suite.Run(t, &TrackerSuite{
		Assertions: require.New(t),
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, "Builds Suite")
}
