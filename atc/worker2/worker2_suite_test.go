package worker2_test

import (
	"testing"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/postgresrunner"
	"github.com/concourse/concourse/atc/worker2/workertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	postgresRunner postgresrunner.Runner
	dbConn         db.Conn
	lockFactory    lock.LockFactory
)

var logger = lagertest.NewTestLogger("dummy")

var _ = postgresrunner.GinkgoRunner(&postgresRunner)

var _ = BeforeEach(func() {
	postgresRunner.CreateTestDBFromTemplate()

	dbConn = postgresRunner.OpenConn()

	ignore := func(logger lager.Logger, id lock.LockID) {}
	lockFactory = lock.NewLockFactory(postgresRunner.OpenSingleton(), ignore, ignore)
})

var _ = AfterEach(func() {
	Expect(dbConn.Close()).To(Succeed())
	postgresRunner.DropTestDB()
})

func TestWorker2(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Worker2 Suite")
}

func Setup(setup ...workertest.SetupFunc) *workertest.Scenario {
	return workertest.Setup(dbConn, lockFactory, setup...)
}

var Test = It
var FTest = FIt
var XTest = XIt
