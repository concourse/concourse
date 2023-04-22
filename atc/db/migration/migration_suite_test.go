package migration_test

import (
	"testing"

	"github.com/concourse/concourse/atc/postgresrunner"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMigration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Migration Suite")
}

var postgresRunner postgresrunner.Runner

var _ = postgresrunner.GinkgoRunner(&postgresRunner)

var _ = BeforeEach(func() {
	postgresRunner.CreateEmptyTestDB()
})

var _ = AfterEach(func() {
	postgresRunner.DropTestDB()
})
