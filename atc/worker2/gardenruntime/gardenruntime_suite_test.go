package gardenruntime_test

import (
	"testing"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbtest"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/postgresrunner"
	"github.com/concourse/concourse/atc/worker2"
	"github.com/concourse/concourse/atc/worker2/workertest"
	"github.com/cppforlife/go-semi-semantic/version"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	postgresRunner postgresrunner.Runner
	dbConn         db.Conn
	lockFactory    lock.LockFactory
	builder        dbtest.Builder
)

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

func TestGardenRuntime(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Garden Runtime Suite")
}

func Setup(setup ...workertest.SetupFunc) *workertest.Scenario {
	poolFactory := func(factory worker2.Factory) worker2.Pool {
		return worker2.Pool{
			Factory: factory,
			DB: worker2.DB{
				WorkerFactory:                 db.NewWorkerFactory(dbConn),
				TeamFactory:                   db.NewTeamFactory(dbConn, lockFactory),
				VolumeRepo:                    db.NewVolumeRepository(dbConn),
				TaskCacheFactory:              db.NewTaskCacheFactory(dbConn),
				WorkerBaseResourceTypeFactory: db.NewWorkerBaseResourceTypeFactory(dbConn),
				LockFactory:                   lockFactory,
			},
			WorkerVersion: version.MustNewVersionFromString(concourse.WorkerVersion),
		}
	}
	builder = dbtest.NewBuilder(dbConn, lockFactory)
	return workertest.Setup(poolFactory, builder, setup...)
}

var Test = It
var FTest = FIt
var XTest = XIt
