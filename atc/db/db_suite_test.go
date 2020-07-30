package db_test

import (
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	sq "github.com/Masterminds/squirrel"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/postgresrunner"
	"github.com/tedsuo/ifrit"
)

func TestDB(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DB Suite")
}

var (
	postgresRunner postgresrunner.Runner
	dbProcess      ifrit.Process

	dbConn                              db.Conn
	fakeSecrets                         *credsfakes.FakeSecrets
	fakeVarSourcePool                   *credsfakes.FakeVarSourcePool
	componentFactory                    db.ComponentFactory
	buildFactory                        db.BuildFactory
	volumeRepository                    db.VolumeRepository
	containerRepository                 db.ContainerRepository
	teamFactory                         db.TeamFactory
	workerFactory                       db.WorkerFactory
	workerLifecycle                     db.WorkerLifecycle
	resourceConfigCheckSessionLifecycle db.ResourceConfigCheckSessionLifecycle
	resourceConfigFactory               db.ResourceConfigFactory
	resourceCacheFactory                db.ResourceCacheFactory
	taskCacheFactory                    db.TaskCacheFactory
	checkFactory                        db.CheckFactory
	workerBaseResourceTypeFactory       db.WorkerBaseResourceTypeFactory
	workerTaskCacheFactory              db.WorkerTaskCacheFactory
	userFactory                         db.UserFactory
	dbWall                              db.Wall
	fakeClock                           dbfakes.FakeClock

	defaultWorkerResourceType atc.WorkerResourceType
	defaultTeam               db.Team
	defaultWorkerPayload      atc.Worker
	defaultWorker             db.Worker
	otherWorker               db.Worker
	otherWorkerPayload        atc.Worker
	defaultResourceType       db.ResourceType
	defaultResource           db.Resource
	defaultPipelineConfig     atc.Config
	defaultPipeline           db.Pipeline
	defaultJob                db.Job
	logger                    *lagertest.TestLogger
	lockFactory               lock.LockFactory

	fullMetadata = db.ContainerMetadata{
		Type: db.ContainerTypeTask,

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
	postgresRunner.Truncate()

	dbConn = postgresRunner.OpenConn()

	lockFactory = lock.NewLockFactory(postgresRunner.OpenSingleton(), metric.LogLockAcquired, metric.LogLockReleased)

	fakeSecrets = new(credsfakes.FakeSecrets)
	fakeVarSourcePool = new(credsfakes.FakeVarSourcePool)
	componentFactory = db.NewComponentFactory(dbConn)
	buildFactory = db.NewBuildFactory(dbConn, lockFactory, 5*time.Minute, 5*time.Minute)
	volumeRepository = db.NewVolumeRepository(dbConn)
	containerRepository = db.NewContainerRepository(dbConn)
	teamFactory = db.NewTeamFactory(dbConn, lockFactory)
	workerFactory = db.NewWorkerFactory(dbConn)
	workerLifecycle = db.NewWorkerLifecycle(dbConn)
	resourceConfigCheckSessionLifecycle = db.NewResourceConfigCheckSessionLifecycle(dbConn)
	resourceConfigFactory = db.NewResourceConfigFactory(dbConn, lockFactory)
	resourceCacheFactory = db.NewResourceCacheFactory(dbConn, lockFactory)
	taskCacheFactory = db.NewTaskCacheFactory(dbConn)
	checkFactory = db.NewCheckFactory(dbConn, lockFactory, fakeSecrets, fakeVarSourcePool, time.Minute)
	workerBaseResourceTypeFactory = db.NewWorkerBaseResourceTypeFactory(dbConn)
	workerTaskCacheFactory = db.NewWorkerTaskCacheFactory(dbConn)
	userFactory = db.NewUserFactory(dbConn)
	dbWall = db.NewWall(dbConn, &fakeClock)

	var err error
	defaultTeam, err = teamFactory.CreateTeam(atc.Team{Name: "default-team"})
	Expect(err).NotTo(HaveOccurred())

	defaultWorkerResourceType = atc.WorkerResourceType{
		Type:    "some-base-resource-type",
		Image:   "/path/to/image",
		Version: "some-brt-version",
	}

	certsPath := "/etc/ssl/certs"

	defaultWorkerPayload = atc.Worker{
		ResourceTypes:   []atc.WorkerResourceType{defaultWorkerResourceType},
		Name:            "default-worker",
		GardenAddr:      "1.2.3.4:7777",
		BaggageclaimURL: "5.6.7.8:7878",
		CertsPath:       &certsPath,
	}

	otherWorkerPayload = atc.Worker{
		ResourceTypes:   []atc.WorkerResourceType{defaultWorkerResourceType},
		Name:            "other-worker",
		GardenAddr:      "2.3.4.5:7777",
		BaggageclaimURL: "6.7.8.9:7878",
		CertsPath:       &certsPath,
	}

	defaultWorker, err = workerFactory.SaveWorker(defaultWorkerPayload, 0)
	Expect(err).NotTo(HaveOccurred())

	otherWorker, err = workerFactory.SaveWorker(otherWorkerPayload, 0)
	Expect(err).NotTo(HaveOccurred())

	defaultPipelineConfig = atc.Config{
		Jobs: atc.JobConfigs{
			{
				Name: "some-job",
			},
		},
		Resources: atc.ResourceConfigs{
			{
				Name: "some-resource",
				Type: "some-base-resource-type",
				Source: atc.Source{
					"some": "source",
				},
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
	}

	defaultPipeline, _, err = defaultTeam.SavePipeline("default-pipeline", defaultPipelineConfig, db.ConfigVersion(0), false)
	Expect(err).NotTo(HaveOccurred())

	var found bool
	defaultResourceType, found, err = defaultPipeline.ResourceType("some-type")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())

	defaultResource, found, err = defaultPipeline.Resource("some-resource")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())

	defaultJob, found, err = defaultPipeline.Job("some-job")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())

	logger = lagertest.NewTestLogger("test")
})

var _ = AfterEach(func() {
	err := dbConn.Close()
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	dbProcess.Signal(os.Interrupt)
	<-dbProcess.Wait()
})
