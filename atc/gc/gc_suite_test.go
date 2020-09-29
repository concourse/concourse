package gc_test

import (
	"context"
	"os"
	"time"

	"code.cloudfoundry.org/lager"

	"code.cloudfoundry.org/lager/lagertest"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/postgresrunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"

	"testing"
)

type GcCollector interface {
	Run(context.Context) error
}

func TestGc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gc Suite")
}

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

var (
	postgresRunner postgresrunner.Runner
	dbProcess      ifrit.Process

	dbConn                 db.Conn
	err                    error
	resourceCacheFactory   db.ResourceCacheFactory
	resourceCacheLifecycle db.ResourceCacheLifecycle
	resourceConfigFactory  db.ResourceConfigFactory
	buildFactory           db.BuildFactory
	lockFactory            lock.LockFactory

	teamFactory db.TeamFactory

	defaultTeam        db.Team
	defaultPipeline    db.Pipeline
	defaultPipelineRef atc.PipelineRef
	defaultJob         db.Job
	defaultBuild       db.Build

	usedResource     db.Resource
	usedResourceType db.ResourceType
	logger           *lagertest.TestLogger
	fakeLogFunc      = func(logger lager.Logger, id lock.LockID) {}
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

	lockFactory = lock.NewLockFactory(postgresRunner.OpenSingleton(), fakeLogFunc, fakeLogFunc)

	teamFactory = db.NewTeamFactory(dbConn, lockFactory)
	buildFactory = db.NewBuildFactory(dbConn, lockFactory, 0, time.Hour)

	defaultTeam, err = teamFactory.CreateTeam(atc.Team{Name: "default-team"})
	Expect(err).NotTo(HaveOccurred())

	defaultBuild, err = defaultTeam.CreateOneOffBuild()
	Expect(err).NotTo(HaveOccurred())

	atcConfig := atc.Config{
		Resources: atc.ResourceConfigs{
			{
				Name:   "some-resource",
				Type:   "some-base-type",
				Source: atc.Source{"some": "source"},
			},
		},
		ResourceTypes: atc.ResourceTypes{
			{
				Name:   "some-resource-type",
				Type:   "some-base-type",
				Source: atc.Source{"some": "source-type"},
			},
		},
		Jobs: atc.JobConfigs{
			{
				Name: "some-job",
			},
			{
				Name: "some-other-job",
			},
		},
	}

	defaultPipelineRef = atc.PipelineRef{Name: "default-pipeline"}
	defaultPipeline, _, err = defaultTeam.SavePipeline(defaultPipelineRef, atcConfig, db.ConfigVersion(0), false)
	Expect(err).NotTo(HaveOccurred())

	var found bool
	defaultJob, found, err = defaultPipeline.Job("some-job")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())

	usedResource, found, err = defaultPipeline.Resource("some-resource")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())

	usedResourceType, found, err = defaultPipeline.ResourceType("some-resource-type")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())

	setupTx, err := dbConn.Begin()
	Expect(err).ToNot(HaveOccurred())

	baseResourceType := db.BaseResourceType{
		Name: "some-base-type",
	}

	_, err = baseResourceType.FindOrCreate(setupTx, false)
	Expect(err).NotTo(HaveOccurred())

	Expect(setupTx.Commit()).To(Succeed())

	logger = lagertest.NewTestLogger("gc-test")

	resourceCacheLifecycle = db.NewResourceCacheLifecycle(dbConn)
	resourceCacheFactory = db.NewResourceCacheFactory(dbConn, lockFactory)
	resourceConfigFactory = db.NewResourceConfigFactory(dbConn, lockFactory)
})

var _ = AfterEach(func() {
	Expect(dbConn.Close()).To(Succeed())
})

var _ = AfterSuite(func() {
	dbProcess.Signal(os.Interrupt)
	Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())
})
