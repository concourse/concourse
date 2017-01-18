package dbng_test

import (
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	sq "github.com/Masterminds/squirrel"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"database/sql"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/lock/lockfakes"
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

	sqlDB                   *sql.DB
	dbConn                  dbng.Conn
	volumeFactory           dbng.VolumeFactory
	containerFactory        dbng.ContainerFactory
	teamFactory             dbng.TeamFactory
	workerFactory           dbng.WorkerFactory
	resourceConfigFactory   dbng.ResourceConfigFactory
	resourceTypeFactory     dbng.ResourceTypeFactory
	resourceCacheFactory    dbng.ResourceCacheFactory
	baseResourceTypeFactory dbng.BaseResourceTypeFactory

	defaultTeam              dbng.Team
	defaultWorker            *dbng.Worker
	defaultResourceConfig    *dbng.UsedResourceConfig
	defaultResourceType      dbng.ResourceType
	defaultResource          *dbng.Resource
	defaultPipeline          dbng.Pipeline
	defaultBuild             *dbng.Build
	defaultCreatingContainer dbng.CreatingContainer
	defaultCreatedContainer  dbng.CreatedContainer
	logger                   *lagertest.TestLogger
	lockFactory              lock.LockFactory

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
	sqlDB = postgresRunner.Open()

	dbConn = dbng.Wrap(sqlDB)

	pgxConn := postgresRunner.OpenPgx()
	fakeConnector := new(lockfakes.FakeConnector)
	retryableConn := &lock.RetryableConn{Connector: fakeConnector, Conn: pgxConn}

	volumeFactory = dbng.NewVolumeFactory(dbConn)
	containerFactory = dbng.NewContainerFactory(dbConn)
	teamFactory = dbng.NewTeamFactory(dbConn)
	workerFactory = dbng.NewWorkerFactory(dbConn)
	lockFactory = lock.NewLockFactory(retryableConn)
	resourceConfigFactory = dbng.NewResourceConfigFactory(dbConn, lockFactory)
	resourceTypeFactory = dbng.NewResourceTypeFactory(dbConn)
	resourceCacheFactory = dbng.NewResourceCacheFactory(dbConn, lockFactory)
	baseResourceTypeFactory = dbng.NewBaseResourceTypeFactory(dbConn)
	teamFactory = dbng.NewTeamFactory(dbConn)

	defaultTeam, err = teamFactory.CreateTeam("default-team")
	Expect(err).NotTo(HaveOccurred())

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

	defaultPipeline, _, err = defaultTeam.SavePipeline("default-pipeline", atc.Config{
		Jobs: atc.JobConfigs{
			{
				Name: "some-job",
			},
		},
	}, dbng.ConfigVersion(0), dbng.PipelineUnpaused)
	Expect(err).NotTo(HaveOccurred())

	defaultBuild, err = defaultTeam.CreateOneOffBuild()
	Expect(err).NotTo(HaveOccurred())

	defaultResource, err = defaultPipeline.CreateResource("default-resource", "{\"resource\":\"config\"}")
	Expect(err).NotTo(HaveOccurred())

	logger = lagertest.NewTestLogger("test")
	defaultResourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfigForResource(logger, defaultResource, "some-base-resource-type", atc.Source{}, defaultPipeline.ID(), atc.ResourceTypes{})
	Expect(err).NotTo(HaveOccurred())

	defaultCreatingContainer, err = defaultTeam.CreateResourceCheckContainer(defaultWorker, defaultResourceConfig)
	Expect(err).NotTo(HaveOccurred())

	defaultCreatedContainer, err = defaultCreatingContainer.Created()
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
