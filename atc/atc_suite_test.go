package atc_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"testing"
)

func TestATC(t *testing.T) {
	suite.Run(t, &StepsSuite{
		Assertions: require.New(t),
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, "ATC Suite")
}
