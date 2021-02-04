package migration_test

import (
	"github.com/concourse/concourse/atc/postgresrunner"
	"github.com/gobuffalo/packr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
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

var asset = packr.NewBox("./migrations").MustBytes
