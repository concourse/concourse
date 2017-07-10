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
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/creds/credsfakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"

	"github.com/concourse/atc/api/jobserver/jobserverfakes"
	"github.com/concourse/atc/api/resourceserver/resourceserverfakes"
	"github.com/concourse/atc/auth/authfakes"
	"github.com/concourse/atc/engine/enginefakes"
	"github.com/concourse/atc/worker/workerfakes"
	"github.com/concourse/atc/wrappa"
)

var (
	sink *lager.ReconfigurableSink

	externalURL  = "https://example.com"
	oAuthBaseURL = "https://oauth.example.com"

	jwtValidator                  *authfakes.FakeValidator
	getTokenValidator             *authfakes.FakeValidator
	userContextReader             *authfakes.FakeUserContextReader
	fakeAuthTokenGenerator        *authfakes.FakeAuthTokenGenerator
	fakeCSRFTokenGenerator        *authfakes.FakeCSRFTokenGenerator
	providerFactory               *authfakes.FakeProviderFactory
	fakeEngine                    *enginefakes.FakeEngine
	fakeWorkerClient              *workerfakes.FakeClient
	fakeVolumeFactory             *dbfakes.FakeVolumeFactory
	fakeContainerFactory          *dbfakes.FakeContainerFactory
	dbTeamFactory                 *dbfakes.FakeTeamFactory
	dbPipelineFactory             *dbfakes.FakePipelineFactory
	fakePipeline                  *dbfakes.FakePipeline
	dbWorkerFactory               *dbfakes.FakeWorkerFactory
	dbWorkerLifecycle             *dbfakes.FakeWorkerLifecycle
	build                         *dbfakes.FakeBuild
	dbBuildFactory                *dbfakes.FakeBuildFactory
	dbTeam                        *dbfakes.FakeTeam
	fakeSchedulerFactory          *jobserverfakes.FakeSchedulerFactory
	fakeScannerFactory            *resourceserverfakes.FakeScannerFactory
	fakeVariablesFactory          *credsfakes.FakeVariablesFactory
	configValidationErrorMessages []string
	peerAddr                      string
	drain                         chan struct{}
	expire                        time.Duration
	isTLSEnabled                  bool
	cliDownloadsDir               string
	logger                        *lagertest.TestLogger

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
		w.Write([]byte("fake event handler factory was here"))
	})
}

var _ = BeforeEach(func() {
	dbTeamFactory = new(dbfakes.FakeTeamFactory)
	dbPipelineFactory = new(dbfakes.FakePipelineFactory)
	dbBuildFactory = new(dbfakes.FakeBuildFactory)

	dbTeam = new(dbfakes.FakeTeam)
	dbTeam.IDReturns(734)
	dbTeamFactory.FindTeamReturns(dbTeam, true, nil)
	dbTeamFactory.GetByIDReturns(dbTeam)

	fakePipeline = new(dbfakes.FakePipeline)
	dbTeam.PipelineReturns(fakePipeline, true, nil)

	dbWorkerFactory = new(dbfakes.FakeWorkerFactory)
	dbWorkerLifecycle = new(dbfakes.FakeWorkerLifecycle)

	jwtValidator = new(authfakes.FakeValidator)
	getTokenValidator = new(authfakes.FakeValidator)

	userContextReader = new(authfakes.FakeUserContextReader)
	fakeAuthTokenGenerator = new(authfakes.FakeAuthTokenGenerator)
	fakeCSRFTokenGenerator = new(authfakes.FakeCSRFTokenGenerator)
	providerFactory = new(authfakes.FakeProviderFactory)

	peerAddr = "127.0.0.1:1234"
	drain = make(chan struct{})

	fakeEngine = new(enginefakes.FakeEngine)
	fakeWorkerClient = new(workerfakes.FakeClient)

	fakeSchedulerFactory = new(jobserverfakes.FakeSchedulerFactory)
	fakeScannerFactory = new(resourceserverfakes.FakeScannerFactory)

	fakeVolumeFactory = new(dbfakes.FakeVolumeFactory)
	fakeContainerFactory = new(dbfakes.FakeContainerFactory)

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
			getTokenValidator,
			userContextReader,
			checkPipelineAccessHandlerFactory,
			checkBuildReadAccessHandlerFactory,
			checkBuildWriteAccessHandlerFactory,
			checkWorkerTeamAccessHandlerFactory,
		),

		fakeAuthTokenGenerator,
		fakeCSRFTokenGenerator,
		providerFactory,
		oAuthBaseURL,

		dbTeamFactory,
		dbPipelineFactory,
		dbWorkerFactory,
		fakeVolumeFactory,
		fakeContainerFactory,
		dbBuildFactory,

		peerAddr,
		constructedEventHandler.Construct,
		drain,

		fakeEngine,
		fakeWorkerClient,

		fakeSchedulerFactory,
		fakeScannerFactory,

		sink,

		expire,

		isTLSEnabled,

		cliDownloadsDir,
		"1.2.3",
		"4.5.6",
		fakeVariablesFactory,
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
