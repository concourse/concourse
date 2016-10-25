package dbng_test

import (
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

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

	dbConn                  dbng.Conn
	volumeFactory           dbng.VolumeFactory
	containerFactory        *dbng.ContainerFactory
	teamFactory             dbng.TeamFactory
	workerFactory           dbng.WorkerFactory
	resourceConfigFactory   dbng.ResourceConfigFactory
	resourceTypeFactory     dbng.ResourceTypeFactory
	baseResourceTypeFactory dbng.BaseResourceTypeFactory
	resourceFactory         *dbng.ResourceFactory
	pipelineFactory         *dbng.PipelineFactory

	defaultTeam           *dbng.Team
	defaultWorker         *dbng.Worker
	defaultResourceConfig *dbng.UsedResourceConfig
	// defaultUsedResourceType  *dbng.UsedResourceType
	defaultResourceType      dbng.ResourceType
	defaultResource          *dbng.Resource
	defaultPipeline          *dbng.Pipeline
	deafultCreatingContainer *dbng.CreatingContainer
	defaultCreatedContainer  *dbng.CreatedContainer
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

	volumeFactory = dbng.NewVolumeFactory(dbConn)
	containerFactory = dbng.NewContainerFactory(dbConn)
	teamFactory = dbng.NewTeamFactory(dbConn)
	workerFactory = dbng.NewWorkerFactory(dbConn)
	resourceConfigFactory = dbng.NewResourceConfigFactory(dbConn)
	resourceTypeFactory = dbng.NewResourceTypeFactory(dbConn)
	baseResourceTypeFactory = dbng.NewBaseResourceTypeFactory(dbConn)
	resourceFactory = dbng.NewResourceFactory(dbConn)
	pipelineFactory = dbng.NewPipelineFactory(dbConn)

	defaultTeam, err = teamFactory.CreateTeam("default-team")
	Expect(err).NotTo(HaveOccurred())

	baseResourceType := atc.WorkerResourceType{
		Type:    "some-base-resource-type",
		Image:   "/path/to/image",
		Version: "some-brt-version",
	}

	atcWorker := atc.Worker{
		ResourceTypes: []atc.WorkerResourceType{baseResourceType},
		Name:          "default-worker",
	}
	defaultWorker, err = workerFactory.SaveWorker(atcWorker, 0)
	Expect(err).NotTo(HaveOccurred())

	defaultPipeline, err = pipelineFactory.CreatePipeline(defaultTeam, "default-pipeline", "some-config")
	Expect(err).NotTo(HaveOccurred())

	defaultResource, err = resourceFactory.CreateResource(defaultPipeline, "default-resource", "{\"resource\":\"config\"}")
	Expect(err).NotTo(HaveOccurred())

	defaultResourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfigForResource(defaultResource, "some-base-resource-type", atc.Source{}, []dbng.ResourceType{})
	Expect(err).NotTo(HaveOccurred())

	deafultCreatingContainer, err = containerFactory.CreateResourceCheckContainer(defaultWorker, defaultResourceConfig, "check-my-stuff")
	Expect(err).NotTo(HaveOccurred())

	defaultCreatedContainer, err = containerFactory.ContainerCreated(deafultCreatingContainer, "some-garden-handle")
	Expect(err).NotTo(HaveOccurred())

})

var _ = AfterEach(func() {
	dbConn.Close()
})

var _ = AfterSuite(func() {
	dbProcess.Signal(os.Interrupt)
	Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())
})
