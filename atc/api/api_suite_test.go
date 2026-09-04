package api_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"

	"github.com/concourse/concourse/v8/atc/api"
	"github.com/concourse/concourse/v8/atc/api/accessor"
	"github.com/concourse/concourse/v8/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/v8/atc/api/apifakes"
	"github.com/concourse/concourse/v8/atc/api/auth"
	"github.com/concourse/concourse/v8/atc/api/containerserver/containerserverfakes"
	"github.com/concourse/concourse/v8/atc/api/policychecker/policycheckerfakes"
	"github.com/concourse/concourse/v8/atc/auditor/auditorfakes"
	"github.com/concourse/concourse/v8/atc/creds"
	"github.com/concourse/concourse/v8/atc/creds/credsfakes"
	"github.com/concourse/concourse/v8/atc/db"
	"github.com/concourse/concourse/v8/atc/db/dbfakes"
	"github.com/concourse/concourse/v8/atc/gc/gcfakes"
	"github.com/concourse/concourse/v8/atc/policy"
	"github.com/concourse/concourse/v8/atc/wrappa"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	sink *lager.ReconfigurableSink

	externalURL      = "https://example.com"
	clusterName      = "Test Cluster"
	featureFlagsJson = ` {
	"build_rerun": false,
	"cache_streamed_volumes": false,
	"global_resources": false,
	"resource_causality": false
}`

	fakeWorkerPool          *apifakes.FakePool
	fakeVolumeRepository    *dbfakes.FakeVolumeRepository
	fakeContainerRepository *dbfakes.FakeContainerRepository
	fakeDestroyer           *gcfakes.FakeDestroyer
	dbTeamFactory           *dbfakes.FakeTeamFactory
	dbPipelineFactory       *dbfakes.FakePipelineFactory
	dbJobFactory            *dbfakes.FakeJobFactory
	dbResourceFactory       *dbfakes.FakeResourceFactory
	dbResourceConfigFactory *dbfakes.FakeResourceConfigFactory
	fakePipeline            *dbfakes.FakePipeline
	fakeAccess              *accessorfakes.FakeAccess
	fakeAccessor            *accessorfakes.FakeAccessFactory
	dbWorkerFactory         *dbfakes.FakeWorkerFactory
	dbWorkerTeamFactory     *dbfakes.FakeTeamFactory
	dbWorkerLifecycle       *dbfakes.FakeWorkerLifecycle
	fakeDbConn              *dbfakes.FakeDbConn
	build                   *dbfakes.FakeBuild
	dbBuildFactory          *dbfakes.FakeBuildFactory
	dbUserFactory           *dbfakes.FakeUserFactory
	dbCheckFactory          *dbfakes.FakeCheckFactory
	dbTeam                  *dbfakes.FakeTeam
	dbWall                  *dbfakes.FakeWall
	dbComponentFactory      *dbfakes.FakeComponentFactory
	fakeSecretManager       *credsfakes.FakeSecrets
	fakeVarSourcePool       *credsfakes.FakeVarSourcePool
	fakePolicyChecker       *policycheckerfakes.FakePolicyChecker
	credsManagers           creds.Managers
	interceptTimeoutFactory *containerserverfakes.FakeInterceptTimeoutFactory
	interceptTimeout        *containerserverfakes.FakeInterceptTimeout
	isTLSEnabled            bool
	cliDownloadsDir         string
	logger                  *lagertest.TestLogger
	fakeClock               *fakeclock.FakeClock
	dbSigningKeyFactory     *dbfakes.FakeSigningKeyFactory

	constructedEventHandler *fakeEventHandlerFactory

	server *httptest.Server
	client *http.Client
)

func TestAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Suite")
}

type fakeEventHandlerFactory struct {
	build db.BuildForAPI

	lock sync.Mutex
}

func (f *fakeEventHandlerFactory) Construct(
	logger lager.Logger,
	build db.BuildForAPI,
) http.Handler {
	f.lock.Lock()
	f.build = build
	f.lock.Unlock()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fake event handler factory was here"))
	})
}

var _ = BeforeEach(func() {
	dbTeamFactory = new(dbfakes.FakeTeamFactory)
	dbWorkerTeamFactory = new(dbfakes.FakeTeamFactory)
	dbPipelineFactory = new(dbfakes.FakePipelineFactory)
	dbJobFactory = new(dbfakes.FakeJobFactory)
	dbResourceFactory = new(dbfakes.FakeResourceFactory)
	dbResourceConfigFactory = new(dbfakes.FakeResourceConfigFactory)
	dbBuildFactory = new(dbfakes.FakeBuildFactory)
	dbUserFactory = new(dbfakes.FakeUserFactory)
	dbCheckFactory = new(dbfakes.FakeCheckFactory)
	dbWall = new(dbfakes.FakeWall)
	dbComponentFactory = new(dbfakes.FakeComponentFactory)
	dbSigningKeyFactory = new(dbfakes.FakeSigningKeyFactory)

	interceptTimeoutFactory = new(containerserverfakes.FakeInterceptTimeoutFactory)
	interceptTimeout = new(containerserverfakes.FakeInterceptTimeout)
	interceptTimeoutFactory.NewInterceptTimeoutReturns(interceptTimeout)

	dbTeam = new(dbfakes.FakeTeam)
	dbTeam.IDReturns(734)
	dbTeamFactory.FindTeamReturns(dbTeam, true, nil)
	dbTeamFactory.GetByIDReturns(dbTeam)
	dbWorkerTeamFactory.FindTeamReturns(dbTeam, true, nil)
	dbWorkerTeamFactory.GetByIDReturns(dbTeam)

	fakeAccess = new(accessorfakes.FakeAccess)
	fakeAccessor = new(accessorfakes.FakeAccessFactory)
	fakeAccessor.CreateReturns(fakeAccess, nil)

	fakePipeline = new(dbfakes.FakePipeline)
	dbTeam.PipelineReturns(fakePipeline, true, nil)

	dbWorkerFactory = new(dbfakes.FakeWorkerFactory)
	dbWorkerLifecycle = new(dbfakes.FakeWorkerLifecycle)
	fakeDbConn = new(dbfakes.FakeDbConn)

	fakeWorkerPool = new(apifakes.FakePool)

	fakeVolumeRepository = new(dbfakes.FakeVolumeRepository)
	fakeContainerRepository = new(dbfakes.FakeContainerRepository)
	fakeDestroyer = new(gcfakes.FakeDestroyer)

	fakeSecretManager = new(credsfakes.FakeSecrets)
	fakeVarSourcePool = new(credsfakes.FakeVarSourcePool)
	credsManagers = make(creds.Managers)

	fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))

	var err error
	cliDownloadsDir, err = os.MkdirTemp("", "cli-downloads")
	Expect(err).NotTo(HaveOccurred())

	constructedEventHandler = &fakeEventHandlerFactory{}

	logger = lagertest.NewTestLogger("api")

	sink = lager.NewReconfigurableSink(lager.NewPrettySink(GinkgoWriter, lager.DEBUG), lager.DEBUG)

	isTLSEnabled = false

	build = new(dbfakes.FakeBuild)

	fakePolicyChecker = new(policycheckerfakes.FakePolicyChecker)
	fakePolicyChecker.CheckReturns(policy.PassedPolicyCheck(), nil)

	server = newApiTestServer()

	client = &http.Client{
		Transport: &http.Transport{},
	}
})

var _ = AfterEach(func() {
	os.Remove(cliDownloadsDir)
	server.Close()
})

func newApiTestServer(opts ...apiConfigOpt) *httptest.Server {
	testServerConfig := &apiServerConfig{
		workerCount:              1,
		componentStaleMultiplier: 2.0,
		concourseVersion:         "1.2.3",
		workerVersion:            "4.5.6",
		interceptUpdateInterval:  time.Second,
	}

	for _, opt := range opts {
		opt(testServerConfig)
	}

	checkPipelineAccessHandlerFactory := auth.NewCheckPipelineAccessHandlerFactory(dbTeamFactory)
	checkBuildReadAccessHandlerFactory := auth.NewCheckBuildReadAccessHandlerFactory(dbBuildFactory)
	checkBuildWriteAccessHandlerFactory := auth.NewCheckBuildWriteAccessHandlerFactory(dbBuildFactory)
	checkWorkerTeamAccessHandlerFactory := auth.NewCheckWorkerTeamAccessHandlerFactory(dbWorkerFactory)

	apiWrapper := wrappa.MultiWrappa{
		wrappa.NewPolicyCheckWrappa(logger, fakePolicyChecker),
		wrappa.NewAPIAuthWrappa(
			checkPipelineAccessHandlerFactory,
			checkBuildReadAccessHandlerFactory,
			checkBuildWriteAccessHandlerFactory,
			checkWorkerTeamAccessHandlerFactory,
		),
	}

	handler, err := api.NewHandler(
		logger,

		externalURL,
		"", // OIDC issuer
		clusterName,

		apiWrapper,

		dbTeamFactory,
		dbPipelineFactory,
		dbJobFactory,
		dbResourceFactory,
		dbWorkerFactory,
		dbWorkerTeamFactory,
		fakeVolumeRepository,
		fakeContainerRepository,
		fakeDestroyer,
		dbBuildFactory,
		dbCheckFactory,
		dbResourceConfigFactory,
		dbUserFactory,
		dbComponentFactory,
		fakeDbConn,
		testServerConfig.workerCount,
		testServerConfig.componentStaleMultiplier,

		constructedEventHandler.Construct,

		fakeWorkerPool,

		sink,

		isTLSEnabled,

		cliDownloadsDir,
		testServerConfig.concourseVersion,
		testServerConfig.workerVersion,
		fakeSecretManager,
		fakeVarSourcePool,
		credsManagers,
		interceptTimeoutFactory,
		testServerConfig.interceptUpdateInterval,
		dbWall,
		fakeClock,
		dbSigningKeyFactory,
	)

	Expect(err).NotTo(HaveOccurred())

	accessorHandler := accessor.NewHandler(
		logger,
		"some-action",
		handler,
		fakeAccessor,
		new(auditorfakes.FakeAuditor),
		map[string]string{},
	)

	handler = wrappa.LoggerHandler{
		Logger:  logger,
		Handler: accessorHandler,
	}

	return httptest.NewServer(handler)
}

type apiServerConfig struct {
	workerCount              int
	componentStaleMultiplier float64
	concourseVersion         string
	workerVersion            string
	interceptUpdateInterval  time.Duration
}

type apiConfigOpt func(*apiServerConfig)

func withWorkerCount(count int) apiConfigOpt {
	return func(a *apiServerConfig) {
		a.workerCount = count
	}
}
