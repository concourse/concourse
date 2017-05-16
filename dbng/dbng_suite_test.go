package dbng_test

import (
	"crypto/aes"
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	sq "github.com/Masterminds/squirrel"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/postgresrunner"
	"github.com/tedsuo/ifrit"
)

func TestDB(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DB Suite : The Next Generation")
}

var (
	postgresRunner postgresrunner.Runner
	dbProcess      ifrit.Process

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
	defaultResourceType       dbng.ResourceType
	defaultResource           dbng.Resource
	defaultPipeline           dbng.Pipeline
	logger                    *lagertest.TestLogger
	lockFactory               lock.LockFactory
	key                       dbng.EncryptionStrategy

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
	eBlock, err := aes.NewCipher([]byte("AES256Key-32Characters1234567890"))
	Expect(err).ToNot(HaveOccurred())
	key = dbng.NewEncryptionKey(eBlock)

	postgresRunner.Truncate()

	dbConn = postgresRunner.OpenConn()

	lockFactory = lock.NewLockFactory(postgresRunner.OpenSingleton())

	buildFactory = dbng.NewBuildFactory(dbConn, lockFactory, key)
	volumeFactory = dbng.NewVolumeFactory(dbConn)
	containerFactory = dbng.NewContainerFactory(dbConn)
	teamFactory = dbng.NewTeamFactory(dbConn, lockFactory, key)
	workerFactory = dbng.NewWorkerFactory(dbConn)
	workerLifecycle = dbng.NewWorkerLifecycle(dbConn)
	resourceConfigFactory = dbng.NewResourceConfigFactory(dbConn, lockFactory)
	resourceCacheFactory = dbng.NewResourceCacheFactory(dbConn, lockFactory)
	baseResourceTypeFactory = dbng.NewBaseResourceTypeFactory(dbConn)
	workerBaseResourceTypeFactory = dbng.NewWorkerBaseResourceTypeFactory(dbConn)

	defaultTeam, err = teamFactory.CreateTeam(atc.Team{Name: "default-team"})
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

	defaultResource, found, err = defaultPipeline.Resource("some-resource")
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
	Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())
})
