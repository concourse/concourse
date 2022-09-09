package db_test

import (
	"database/sql"
	"testing"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	sq "github.com/Masterminds/squirrel"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/dbtest"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/postgresrunner"
	"github.com/concourse/concourse/atc/util"
)

func TestDB(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DB Suite")
}

var (
	postgresRunner postgresrunner.Runner

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

	builder dbtest.Builder

	defaultWorkerResourceType atc.WorkerResourceType
	uniqueWorkerResourceType  atc.WorkerResourceType
	defaultTeam               db.Team
	defaultWorkerPayload      atc.Worker
	defaultWorker             db.Worker
	otherWorker               db.Worker
	otherWorkerPayload        atc.Worker
	defaultPrototype          db.Prototype
	defaultResourceType       db.ResourceType
	defaultResource           db.Resource
	defaultPipelineConfig     atc.Config
	defaultPipeline           db.Pipeline
	defaultPipelineRef        atc.PipelineRef
	defaultJob                db.Job
	logger                    *lagertest.TestLogger
	lockFactory               lock.LockFactory

	defaultCheckInterval        = time.Minute
	defaultWebhookCheckInterval = time.Hour
	defaultCheckTimeout         = 5 * time.Minute

	defaultBuildCreatedBy string

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

	checkBuildChan chan db.Build
	seqGenerator   util.SequenceGenerator
)

var _ = postgresrunner.GinkgoRunner(&postgresRunner)

var _ = BeforeEach(func() {
	logger = lagertest.NewTestLogger("test")

	postgresRunner.CreateTestDBFromTemplate()

	dbConn = postgresRunner.OpenConn()
	db.CleanupBaseResourceTypesCache()

	var lockConns [lock.FactoryCount]*sql.DB
	for i := 0; i < lock.FactoryCount; i++ {
		lockConns[i] = postgresRunner.OpenSingleton()
	}
	lockFactory = lock.NewLockFactory(lockConns, metric.LogLockAcquired, metric.LogLockReleased)

	checkBuildChan = make(chan db.Build, 10)
	seqGenerator = util.NewSequenceGenerator(1)

	fakeSecrets = new(credsfakes.FakeSecrets)
	fakeVarSourcePool = new(credsfakes.FakeVarSourcePool)
	componentFactory = db.NewComponentFactory(dbConn)
	buildFactory = db.NewBuildFactory(dbConn, lockFactory, 5*time.Minute, 5*time.Minute)
	volumeRepository = db.NewVolumeRepository(dbConn)
	containerRepository = db.NewContainerRepository(dbConn)
	teamFactory = db.NewTeamFactory(dbConn, lockFactory)
	workerFactory = db.NewWorkerFactory(dbConn, db.NewStaticWorkerCache(logger, dbConn, 0))
	workerLifecycle = db.NewWorkerLifecycle(dbConn)
	resourceConfigCheckSessionLifecycle = db.NewResourceConfigCheckSessionLifecycle(dbConn)
	resourceConfigFactory = db.NewResourceConfigFactory(dbConn, lockFactory)
	resourceCacheFactory = db.NewResourceCacheFactory(dbConn, lockFactory)
	taskCacheFactory = db.NewTaskCacheFactory(dbConn)
	checkFactory = db.NewCheckFactory(dbConn, lockFactory, fakeSecrets, fakeVarSourcePool, checkBuildChan, seqGenerator)
	workerBaseResourceTypeFactory = db.NewWorkerBaseResourceTypeFactory(dbConn)
	workerTaskCacheFactory = db.NewWorkerTaskCacheFactory(dbConn)
	userFactory = db.NewUserFactory(dbConn)
	dbWall = db.NewWall(dbConn, &fakeClock)

	builder = dbtest.NewBuilder(dbConn, lockFactory)

	var err error
	defaultTeam, err = teamFactory.CreateTeam(atc.Team{Name: "default-team"})
	Expect(err).NotTo(HaveOccurred())

	defaultWorkerResourceType = atc.WorkerResourceType{
		Type:    "some-base-resource-type",
		Image:   "/path/to/image",
		Version: "some-brt-version",
	}

	uniqueWorkerResourceType = atc.WorkerResourceType{
		Type:                 "some-unique-base-resource-type",
		Image:                "/path/to/unique/image",
		Version:              "some-unique-brt-version",
		UniqueVersionHistory: true,
	}

	certsPath := "/etc/ssl/certs"

	defaultWorkerPayload = atc.Worker{
		Name:            "default-worker",
		GardenAddr:      "1.2.3.4:7777",
		BaggageclaimURL: "5.6.7.8:7878",
		CertsPath:       &certsPath,

		ResourceTypes: []atc.WorkerResourceType{
			defaultWorkerResourceType,
			uniqueWorkerResourceType,
		},
	}

	otherWorkerPayload = atc.Worker{
		Name:            "other-worker",
		GardenAddr:      "2.3.4.5:7777",
		BaggageclaimURL: "6.7.8.9:7878",
		CertsPath:       &certsPath,

		ResourceTypes: []atc.WorkerResourceType{
			defaultWorkerResourceType,
			uniqueWorkerResourceType,
		},
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
		Prototypes: atc.Prototypes{
			{
				Name: "some-prototype",
				Type: "some-base-resource-type",
				Source: atc.Source{
					"some-prototype": "source",
				},
			},
		},
	}

	defaultPipelineRef = atc.PipelineRef{Name: "default-pipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}

	defaultPipeline, _, err = defaultTeam.SavePipeline(defaultPipelineRef, defaultPipelineConfig, db.ConfigVersion(0), false)
	Expect(err).NotTo(HaveOccurred())

	var found bool
	defaultResourceType, found, err = defaultPipeline.ResourceType("some-type")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())

	defaultResource, found, err = defaultPipeline.Resource("some-resource")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())

	defaultPrototype, found, err = defaultPipeline.Prototype("some-prototype")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())

	defaultJob, found, err = defaultPipeline.Job("some-job")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())

	defaultBuildCreatedBy = "some-user"
})

func destroy(d interface{ Destroy() error }) {
	err := d.Destroy()
	Expect(err).ToNot(HaveOccurred())
}

var _ = AfterEach(func() {
	err := dbConn.Close()
	Expect(err).NotTo(HaveOccurred())

	postgresRunner.DropTestDB()
})
