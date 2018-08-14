package gc_test

import (
	"os"
	"time"

	"code.cloudfoundry.org/lager/lagertest"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/postgresrunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"

	"testing"
)

func TestGc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gc Suite")
}

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

var (
	postgresRunner postgresrunner.Runner
	dbProcess      ifrit.Process

	dbConn                            db.Conn
	err                               error
	resourceCacheFactory              db.ResourceCacheFactory
	resourceCacheLifecycle            db.ResourceCacheLifecycle
	resourceConfigFactory             db.ResourceConfigFactory
	resourceConfigCheckSessionFactory db.ResourceConfigCheckSessionFactory
	buildFactory                      db.BuildFactory
	lockFactory                       lock.LockFactory

	teamFactory db.TeamFactory

	defaultTeam     db.Team
	defaultPipeline db.Pipeline
	defaultJob      db.Job
	defaultBuild    db.Build

	usedResource db.Resource
	logger       *lagertest.TestLogger
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

	lockFactory = lock.NewLockFactory(postgresRunner.OpenSingleton())

	teamFactory = db.NewTeamFactory(dbConn, lockFactory)
	buildFactory = db.NewBuildFactory(dbConn, lockFactory, 0)

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
		Jobs: atc.JobConfigs{
			{
				Name: "some-job",
			},
			{
				Name: "some-other-job",
			},
		},
	}

	defaultPipeline, _, err = defaultTeam.SavePipeline("default-pipeline", atcConfig, db.ConfigVersion(0), db.PipelineUnpaused)
	Expect(err).NotTo(HaveOccurred())

	var found bool
	defaultJob, found, err = defaultPipeline.Job("some-job")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())

	usedResource, found, err = defaultPipeline.Resource("some-resource")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())

	setupTx, err := dbConn.Begin()
	Expect(err).ToNot(HaveOccurred())

	baseResourceType := db.BaseResourceType{
		Name: "some-base-type",
	}
	_, err = baseResourceType.FindOrCreate(setupTx)
	Expect(err).NotTo(HaveOccurred())

	Expect(setupTx.Commit()).To(Succeed())

	logger = lagertest.NewTestLogger("gc-test")

	resourceCacheLifecycle = db.NewResourceCacheLifecycle(dbConn)
	resourceCacheFactory = db.NewResourceCacheFactory(dbConn, lockFactory)
	resourceConfigFactory = db.NewResourceConfigFactory(dbConn, lockFactory)
	resourceConfigCheckSessionFactory = db.NewResourceConfigCheckSessionFactory(dbConn, lockFactory)
})

var _ = AfterEach(func() {
	Expect(dbConn.Close()).To(Succeed())
})

var _ = AfterSuite(func() {
	dbProcess.Signal(os.Interrupt)
	Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())
})
