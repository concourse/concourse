package auth_test

import (
	"testing"

	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAuth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auth Suite")
}

var logger lager.Logger

var _ = BeforeEach(func() {
	logger = lager.NewLogger("test")
})
