package dexserver_test

import (
	"os"

	"github.com/concourse/concourse/atc/postgresrunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"

	"testing"
)

func TestDexServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dex Server Suite")
}

var postgresRunner postgresrunner.Runner
var dbProcess ifrit.Process

var _ = BeforeSuite(func() {
	postgresRunner = postgresrunner.Runner{
		Port: 5433 + GinkgoParallelNode(),
	}
	dbProcess = ifrit.Invoke(postgresRunner)
})

var _ = BeforeEach(func() {
	postgresRunner.CreateTestDB()
})

var _ = AfterEach(func() {
	postgresRunner.DropTestDB()
})

var _ = AfterSuite(func() {
	dbProcess.Signal(os.Interrupt)
	<-dbProcess.Wait()
})
