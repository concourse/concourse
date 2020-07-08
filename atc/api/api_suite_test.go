package api_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/concourse/atc/api"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/api/containerserver/containerserverfakes"
	"github.com/concourse/concourse/atc/api/policychecker/policycheckerfakes"
	"github.com/concourse/concourse/atc/auditor/auditorfakes"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/gc/gcfakes"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/concourse/concourse/atc/wrappa"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	sink *lager.ReconfigurableSink

	externalURL = "https://example.com"
	clusterName = "Test Cluster"

	fakeWorkerClient        *workerfakes.FakeClient
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
	dbWorkerLifecycle       *dbfakes.FakeWorkerLifecycle
	build                   *dbfakes.FakeBuild
	dbBuildFactory          *dbfakes.FakeBuildFactory
	dbUserFactory           *dbfakes.FakeUserFactory
	dbCheckFactory          *dbfakes.FakeCheckFactory
	dbTeam                  *dbfakes.FakeTeam
	dbWall                  *dbfakes.FakeWall
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

	constructedEventHandler *fakeEventHandlerFactory

	server *httptest.Server
	client *http.Client
)

type fakeEventHandlerFactory struct {
	build db.Build

	lock sync.Mutex
}

func (f *fakeEventHandlerFactory) Construct(
	logger lager.Logger,
	build db.Build,
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
	dbPipelineFactory = new(dbfakes.FakePipelineFactory)
	dbJobFactory = new(dbfakes.FakeJobFactory)
	dbResourceFactory = new(dbfakes.FakeResourceFactory)
	dbResourceConfigFactory = new(dbfakes.FakeResourceConfigFactory)
	dbBuildFactory = new(dbfakes.FakeBuildFactory)
	dbUserFactory = new(dbfakes.FakeUserFactory)
	dbCheckFactory = new(dbfakes.FakeCheckFactory)
	dbWall = new(dbfakes.FakeWall)

	interceptTimeoutFactory = new(containerserverfakes.FakeInterceptTimeoutFactory)
	interceptTimeout = new(containerserverfakes.FakeInterceptTimeout)
	interceptTimeoutFactory.NewInterceptTimeoutReturns(interceptTimeout)

	dbTeam = new(dbfakes.FakeTeam)
	dbTeam.IDReturns(734)
	dbTeamFactory.FindTeamReturns(dbTeam, true, nil)
	dbTeamFactory.GetByIDReturns(dbTeam)

	fakeAccess = new(accessorfakes.FakeAccess)
	fakeAccessor = new(accessorfakes.FakeAccessFactory)
	fakeAccessor.CreateReturns(fakeAccess)

	fakePipeline = new(dbfakes.FakePipeline)
	dbTeam.PipelineReturns(fakePipeline, true, nil)

	dbWorkerFactory = new(dbfakes.FakeWorkerFactory)
	dbWorkerLifecycle = new(dbfakes.FakeWorkerLifecycle)

	fakeWorkerClient = new(workerfakes.FakeClient)

	fakeVolumeRepository = new(dbfakes.FakeVolumeRepository)
	fakeContainerRepository = new(dbfakes.FakeContainerRepository)
	fakeDestroyer = new(gcfakes.FakeDestroyer)

	fakeSecretManager = new(credsfakes.FakeSecrets)
	fakeVarSourcePool = new(credsfakes.FakeVarSourcePool)
	credsManagers = make(creds.Managers)

	fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))

	var err error
	cliDownloadsDir, err = ioutil.TempDir("", "cli-downloads")
	Expect(err).NotTo(HaveOccurred())

	constructedEventHandler = &fakeEventHandlerFactory{}

	logger = lagertest.NewTestLogger("api")

	sink = lager.NewReconfigurableSink(lager.NewPrettySink(GinkgoWriter, lager.DEBUG), lager.DEBUG)

	isTLSEnabled = false

	build = new(dbfakes.FakeBuild)

	checkPipelineAccessHandlerFactory := auth.NewCheckPipelineAccessHandlerFactory(dbTeamFactory)

	checkBuildReadAccessHandlerFactory := auth.NewCheckBuildReadAccessHandlerFactory(dbBuildFactory)

	checkBuildWriteAccessHandlerFactory := auth.NewCheckBuildWriteAccessHandlerFactory(dbBuildFactory)

	checkWorkerTeamAccessHandlerFactory := auth.NewCheckWorkerTeamAccessHandlerFactory(dbWorkerFactory)

	fakePolicyChecker = new(policycheckerfakes.FakePolicyChecker)
	fakePolicyChecker.CheckReturns(true, nil)

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
		clusterName,

		apiWrapper,

		dbTeamFactory,
		dbPipelineFactory,
		dbJobFactory,
		dbResourceFactory,
		dbWorkerFactory,
		fakeVolumeRepository,
		fakeContainerRepository,
		fakeDestroyer,
		dbBuildFactory,
		dbCheckFactory,
		dbResourceConfigFactory,
		dbUserFactory,

		constructedEventHandler.Construct,

		fakeWorkerClient,

		sink,

		isTLSEnabled,

		cliDownloadsDir,
		"1.2.3",
		"4.5.6",
		fakeSecretManager,
		fakeVarSourcePool,
		credsManagers,
		interceptTimeoutFactory,
		time.Second,
		dbWall,
		fakeClock,
	)

	Expect(err).NotTo(HaveOccurred())

	accessorHandler := accessor.NewHandler(
		logger,
		"some-action",
		handler,
		fakeAccessor,
		new(accessorfakes.FakeTokenVerifier),
		new(accessorfakes.FakeTeamFetcher),
		new(accessorfakes.FakeUserTracker),
		new(auditorfakes.FakeAuditor),
		map[string]string{},
	)

	handler = wrappa.LoggerHandler{
		Logger:  logger,
		Handler: accessorHandler,
	}

	server = httptest.NewServer(handler)

	client = &http.Client{
		Transport: &http.Transport{},
	}
})

var _ = AfterEach(func() {
	server.Close()
})

func TestAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Suite")
}
