package dexserver_test

import (
	"github.com/concourse/concourse/atc/postgresrunner"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDexServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dex Server Suite")
}

var postgresRunner postgresrunner.Runner

var _ = postgresrunner.GinkgoRunner(&postgresRunner)

var _ = BeforeEach(func() {
	postgresRunner.CreateTestDBFromTemplate()
})

var _ = AfterEach(func() {
	postgresRunner.DropTestDB()
})
