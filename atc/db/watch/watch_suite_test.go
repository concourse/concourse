package watch_test

import (
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/postgresrunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

func TestWatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Watch Suite")
}


var (
	postgresRunner postgresrunner.Runner
	dbProcess ifrit.Process

	dbConn      db.Conn
	lockFactory lock.LockFactory
)

var _ = BeforeSuite(func() {
	postgresRunner = postgresrunner.Runner{
		Port: 5433 + GinkgoParallelNode(),
	}

	dbProcess = ifrit.Invoke(postgresRunner)

	postgresRunner.CreateTestDB()
})

var _ = BeforeEach(func() {
	postgresRunner.Truncate()

	dbConn = postgresRunner.OpenConn()

	dummyLogFunc := func(logger lager.Logger, id lock.LockID) {}
	lockFactory = lock.NewLockFactory(postgresRunner.OpenSingleton(), dummyLogFunc, dummyLogFunc)
})

var _ = AfterEach(func() {
	err := dbConn.Close()
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	dbProcess.Signal(os.Interrupt)
	Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())
})
