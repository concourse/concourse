package dbng_test

import (
	"database/sql"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/postgresrunner"
	"github.com/tedsuo/ifrit"
)

func TestDB(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DB Suite")
}

var (
	err            error
	postgresRunner postgresrunner.Runner
	dbProcess      ifrit.Process

	sqlDB  *sql.DB
	dbConn dbng.Conn

	workerFactory    dbng.WorkerFactory
	teamFactory      dbng.TeamFactory
	containerFactory *dbng.ContainerFactory

	defaultWorker *dbng.Worker
	defaultTeam   dbng.Team

	psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
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
	dbConn = dbng.Wrap(postgresRunner.Open())

	workerFactory = dbng.NewWorkerFactory(dbConn)
	teamFactory = dbng.NewTeamFactory(dbConn)
	containerFactory = dbng.NewContainerFactory(dbConn)

	baseResourceType := atc.WorkerResourceType{
		Type:    "some-base-resource-type",
		Image:   "/path/to/image",
		Version: "some-brt-version",
	}

	atcWorker := atc.Worker{
		ResourceTypes:   []atc.WorkerResourceType{baseResourceType},
		Name:            "default-worker",
		GardenAddr:      "1.2.3.4:7777",
		BaggageclaimURL: "5.6.7.8:7878",
	}

	defaultWorker, err = workerFactory.SaveWorker(atcWorker, 0)
	Expect(err).NotTo(HaveOccurred())

	defaultTeam, err = teamFactory.CreateTeam("some-team")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterEach(func() {
	err := dbConn.Close()
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	dbProcess.Signal(os.Interrupt)
	Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())
})
