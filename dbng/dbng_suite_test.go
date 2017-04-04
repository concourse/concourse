package dbng_test

import (
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	sq "github.com/Masterminds/squirrel"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

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
	RunSpecs(t, "DB Suite : The Next Generation")
}

var (
	err            error
	postgresRunner postgresrunner.Runner
	dbProcess      ifrit.Process

	sqlDB                         *sql.DB
	dbConn                        dbng.Conn
	buildFactory                  dbng.BuildFactory
	volumeFactory                 dbng.VolumeFactory
	containerFactory              dbng.ContainerFactory
	teamFactory                   dbng.TeamFactory
	workerFactory                 dbng.WorkerFactory
	workerLifecycle               dbng.WorkerLifecycle
	resourceConfigFactory         dbng.ResourceConfigFactory
	resourceCacheFactory          dbng.ResourceCacheFactory
	baseResourceTypeFactory       dbng.BaseResourceTypeFactory
	workerBaseResourceTypeFactory dbng.WorkerBaseResourceTypeFactory

	defaultWorkerResourceType atc.WorkerResourceType
	defaultTeam               dbng.Team
	defaultWorkerPayload      atc.Worker
	defaultWorker             dbng.Worker
	defaultResourceConfig     *dbng.UsedResourceConfig
	defaultResourceType       dbng.ResourceType
	defaultResource           *dbng.Resource
	defaultPipeline           dbng.Pipeline
	defaultBuild              dbng.Build
	defaultCreatingContainer  dbng.CreatingContainer
	defaultCreatedContainer   dbng.CreatedContainer
	logger                    *lagertest.TestLogger
	lockFactory               lock.LockFactory

	fullMetadata = dbng.ContainerMetadata{
		Type: dbng.ContainerTypeTask,

		StepName: "some-step-name",
		Attempt:  "1.2.3",

		PipelineID: 123,
		JobID:      456,
		BuildID:    789,

		PipelineName: "some-pipeline",
		JobName:      "some-job",
		BuildName:    "some-build",

		WorkingDirectory: "/some/work/dir",
		User:             "some-user",
	}

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
	format.UseStringerRepresentation = true

	postgresRunner.Truncate()
	sqlDB = postgresRunner.Open()

	dbConn = dbng.Wrap(sqlDB)

	pgxConn := postgresRunner.OpenPgx()
	fakeConnector := new(lockfakes.FakeConnector)
	retryableConn := &lock.RetryableConn{Connector: fakeConnector, Conn: pgxConn}

	buildFactory = dbng.NewBuildFactory(dbConn)
	volumeFactory = dbng.NewVolumeFactory(dbConn)
	containerFactory = dbng.NewContainerFactory(dbConn)
	lockFactory = lock.NewLockFactory(retryableConn)
	teamFactory = dbng.NewTeamFactory(dbConn, lockFactory)
	workerFactory = dbng.NewWorkerFactory(dbConn)
	workerLifecycle = dbng.NewWorkerLifecycle(dbConn)
	resourceConfigFactory = dbng.NewResourceConfigFactory(dbConn, lockFactory)
	resourceCacheFactory = dbng.NewResourceCacheFactory(dbConn, lockFactory)
	baseResourceTypeFactory = dbng.NewBaseResourceTypeFactory(dbConn)
	workerBaseResourceTypeFactory = dbng.NewWorkerBaseResourceTypeFactory(dbConn)

	defaultTeam, err = teamFactory.CreateTeam("default-team")
	Expect(err).NotTo(HaveOccurred())

	defaultWorkerResourceType = atc.WorkerResourceType{
		Type:    "some-base-resource-type",
		Image:   "/path/to/image",
		Version: "some-brt-version",
	}

	defaultWorkerPayload = atc.Worker{
		ResourceTypes:   []atc.WorkerResourceType{defaultWorkerResourceType},
		Name:            "default-worker",
		GardenAddr:      "1.2.3.4:7777",
		BaggageclaimURL: "5.6.7.8:7878",
	}

	defaultWorker, err = workerFactory.SaveWorker(defaultWorkerPayload, 0)
	Expect(err).NotTo(HaveOccurred())

	defaultPipeline, _, err = defaultTeam.SavePipeline("default-pipeline", atc.Config{
		Jobs: atc.JobConfigs{
			{
				Name: "some-job",
			},
		},
		ResourceTypes: atc.ResourceTypes{
			{
				Name: "some-type",
				Type: "some-base-resource-type",
				Source: atc.Source{
					"some-type": "source",
				},
			},
		},
	}, dbng.ConfigVersion(0), dbng.PipelineUnpaused)
	Expect(err).NotTo(HaveOccurred())

	var found bool
	defaultResourceType, found, err = defaultPipeline.ResourceType("some-type")
	Expect(found).To(BeTrue())
	Expect(err).NotTo(HaveOccurred())

	err = defaultResourceType.SaveVersion(atc.Version{"some-type": "version"})
	Expect(err).NotTo(HaveOccurred())

	found, err = defaultResourceType.Reload()
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())

	defaultBuild, err = defaultTeam.CreateOneOffBuild()
	Expect(err).NotTo(HaveOccurred())

	defaultResource, err = defaultPipeline.CreateResource("default-resource", atc.ResourceConfig{})
	Expect(err).NotTo(HaveOccurred())

	logger = lagertest.NewTestLogger("test")
	defaultResourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, dbng.ForResource(defaultResource.ID), "some-base-resource-type", atc.Source{}, atc.VersionedResourceTypes{})
	Expect(err).NotTo(HaveOccurred())

	defaultCreatingContainer, err = defaultTeam.CreateResourceCheckContainer(defaultWorker.Name(), defaultResourceConfig, dbng.ContainerMetadata{Type: "check"})
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
