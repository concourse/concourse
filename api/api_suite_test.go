package api_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/api"
	"github.com/concourse/atc/api/auth"
	"github.com/concourse/atc/creds/credsfakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"

	"github.com/concourse/atc/api/auth/authfakes"
	"github.com/concourse/atc/api/containerserver/containerserverfakes"
	"github.com/concourse/atc/api/jobserver/jobserverfakes"
	"github.com/concourse/atc/api/resourceserver/resourceserverfakes"
	"github.com/concourse/atc/engine/enginefakes"
	"github.com/concourse/atc/worker/workerfakes"
	"github.com/concourse/atc/wrappa"
)

var (
	sink *lager.ReconfigurableSink

	externalURL  = "https://example.com"
	oAuthBaseURL = "https://oauth.example.com"

	jwtValidator            *authfakes.FakeValidator
	userContextReader       *authfakes.FakeUserContextReader
	fakeEngine              *enginefakes.FakeEngine
	fakeWorkerClient        *workerfakes.FakeClient
	fakeWorkerProvider      *workerfakes.FakeWorkerProvider
	fakeVolumeFactory       *dbfakes.FakeVolumeFactory
	fakeContainerRepository *dbfakes.FakeContainerRepository
	dbTeamFactory           *dbfakes.FakeTeamFactory
	dbPipelineFactory       *dbfakes.FakePipelineFactory
	fakePipeline            *dbfakes.FakePipeline
	dbWorkerFactory         *dbfakes.FakeWorkerFactory
	dbWorkerLifecycle       *dbfakes.FakeWorkerLifecycle
	build                   *dbfakes.FakeBuild
	dbBuildFactory          *dbfakes.FakeBuildFactory
	dbTeam                  *dbfakes.FakeTeam
	fakeSchedulerFactory    *jobserverfakes.FakeSchedulerFactory
	fakeScannerFactory      *resourceserverfakes.FakeScannerFactory
	fakeVariablesFactory    *credsfakes.FakeVariablesFactory
	interceptTimeoutFactory *containerserverfakes.FakeInterceptTimeoutFactory
	interceptTimeout        *containerserverfakes.FakeInterceptTimeout
	peerAddr                string
	drain                   chan struct{}
	expire                  time.Duration
	isTLSEnabled            bool
	cliDownloadsDir         string
	logger                  *lagertest.TestLogger

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
	dbBuildFactory = new(dbfakes.FakeBuildFactory)

	interceptTimeoutFactory = new(containerserverfakes.FakeInterceptTimeoutFactory)
	interceptTimeout = new(containerserverfakes.FakeInterceptTimeout)
	interceptTimeoutFactory.NewInterceptTimeoutReturns(interceptTimeout)

	dbTeam = new(dbfakes.FakeTeam)
	dbTeam.IDReturns(734)
	dbTeamFactory.FindTeamReturns(dbTeam, true, nil)
	dbTeamFactory.GetByIDReturns(dbTeam)

	fakePipeline = new(dbfakes.FakePipeline)
	dbTeam.PipelineReturns(fakePipeline, true, nil)

	dbWorkerFactory = new(dbfakes.FakeWorkerFactory)
	dbWorkerLifecycle = new(dbfakes.FakeWorkerLifecycle)

	jwtValidator = new(authfakes.FakeValidator)
	userContextReader = new(authfakes.FakeUserContextReader)

	peerAddr = "127.0.0.1:1234"
	drain = make(chan struct{})

	fakeEngine = new(enginefakes.FakeEngine)
	fakeWorkerClient = new(workerfakes.FakeClient)
	fakeWorkerProvider = new(workerfakes.FakeWorkerProvider)

	fakeSchedulerFactory = new(jobserverfakes.FakeSchedulerFactory)
	fakeScannerFactory = new(resourceserverfakes.FakeScannerFactory)

	fakeVolumeFactory = new(dbfakes.FakeVolumeFactory)
	fakeContainerRepository = new(dbfakes.FakeContainerRepository)

	fakeVariablesFactory = new(credsfakes.FakeVariablesFactory)

	var err error

	cliDownloadsDir, err = ioutil.TempDir("", "cli-downloads")
	Expect(err).NotTo(HaveOccurred())

	constructedEventHandler = &fakeEventHandlerFactory{}

	logger = lagertest.NewTestLogger("api")

	sink = lager.NewReconfigurableSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG), lager.DEBUG)

	expire = 24 * time.Hour

	isTLSEnabled = false

	build = new(dbfakes.FakeBuild)

	checkPipelineAccessHandlerFactory := auth.NewCheckPipelineAccessHandlerFactory(dbTeamFactory)

	checkBuildReadAccessHandlerFactory := auth.NewCheckBuildReadAccessHandlerFactory(dbBuildFactory)

	checkBuildWriteAccessHandlerFactory := auth.NewCheckBuildWriteAccessHandlerFactory(dbBuildFactory)

	checkWorkerTeamAccessHandlerFactory := auth.NewCheckWorkerTeamAccessHandlerFactory(dbWorkerFactory)

	handler, err := api.NewHandler(
		logger,

		externalURL,

		wrappa.NewAPIAuthWrappa(
			jwtValidator,
			userContextReader,
			checkPipelineAccessHandlerFactory,
			checkBuildReadAccessHandlerFactory,
			checkBuildWriteAccessHandlerFactory,
			checkWorkerTeamAccessHandlerFactory,
		),

		oAuthBaseURL,

		dbTeamFactory,
		dbPipelineFactory,
		dbWorkerFactory,
		fakeVolumeFactory,
		fakeContainerRepository,
		dbBuildFactory,

		peerAddr,
		constructedEventHandler.Construct,
		drain,

		fakeEngine,
		fakeWorkerClient,
		fakeWorkerProvider,

		fakeSchedulerFactory,
		fakeScannerFactory,

		sink,

		expire,

		isTLSEnabled,

		cliDownloadsDir,
		"1.2.3",
		"4.5.6",
		fakeVariablesFactory,
		interceptTimeoutFactory,
	)
	Expect(err).NotTo(HaveOccurred())

	handler = wrappa.LoggerHandler{
		Logger:  logger,
		Handler: handler,
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
