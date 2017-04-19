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
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"

	"github.com/concourse/atc/api/jobserver/jobserverfakes"
	"github.com/concourse/atc/api/pipes/pipesfakes"
	"github.com/concourse/atc/api/resourceserver/resourceserverfakes"
	"github.com/concourse/atc/auth/authfakes"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/engine/enginefakes"
	"github.com/concourse/atc/worker/workerfakes"
	"github.com/concourse/atc/wrappa"
)

var (
	sink *lager.ReconfigurableSink

	externalURL  = "https://example.com"
	oAuthBaseURL = "https://oauth.example.com"

	authValidator                 *authfakes.FakeValidator
	userContextReader             *authfakes.FakeUserContextReader
	fakeAuthTokenGenerator        *authfakes.FakeAuthTokenGenerator
	fakeCSRFTokenGenerator        *authfakes.FakeCSRFTokenGenerator
	providerFactory               *authfakes.FakeProviderFactory
	fakeEngine                    *enginefakes.FakeEngine
	fakeWorkerClient              *workerfakes.FakeClient
	fakeVolumeFactory             *dbngfakes.FakeVolumeFactory
	fakeContainerFactory          *dbngfakes.FakeContainerFactory
	pipeDB                        *pipesfakes.FakePipeDB
	pipelineDBFactory             *dbfakes.FakePipelineDBFactory
	teamDBFactory                 *dbfakes.FakeTeamDBFactory
	dbTeamFactory                 *dbngfakes.FakeTeamFactory
	dbPipelineFactory             *dbngfakes.FakePipelineFactory
	fakePipeline                  *dbngfakes.FakePipeline
	dbWorkerFactory               *dbngfakes.FakeWorkerFactory
	dbWorkerLifecycle             *dbngfakes.FakeWorkerLifecycle
	teamDB                        *dbfakes.FakeTeamDB
	pipelinesDB                   *dbfakes.FakePipelinesDB
	build                         *dbfakes.FakeBuild
	dbBuildFactory                *dbngfakes.FakeBuildFactory
	dbTeam                        *dbngfakes.FakeTeam
	fakeSchedulerFactory          *jobserverfakes.FakeSchedulerFactory
	fakeScannerFactory            *resourceserverfakes.FakeScannerFactory
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
	build dbng.Build

	lock sync.Mutex
}

func (f *fakeEventHandlerFactory) Construct(
	logger lager.Logger,
	build dbng.Build,
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
	pipelineDBFactory = new(dbfakes.FakePipelineDBFactory)
	teamDBFactory = new(dbfakes.FakeTeamDBFactory)
	dbTeamFactory = new(dbngfakes.FakeTeamFactory)
	dbPipelineFactory = new(dbngfakes.FakePipelineFactory)
	dbBuildFactory = new(dbngfakes.FakeBuildFactory)

	dbTeam = new(dbngfakes.FakeTeam)
	dbTeam.IDReturns(734)
	dbTeamFactory.FindTeamReturns(dbTeam, true, nil)
	dbTeamFactory.GetByIDReturns(dbTeam)

	fakePipeline = new(dbngfakes.FakePipeline)
	dbTeam.PipelineReturns(fakePipeline, true, nil)

	dbWorkerFactory = new(dbngfakes.FakeWorkerFactory)
	dbWorkerLifecycle = new(dbngfakes.FakeWorkerLifecycle)
	teamDB = new(dbfakes.FakeTeamDB)
	teamDBFactory.GetTeamDBReturns(teamDB)
	pipeDB = new(pipesfakes.FakePipeDB)
	pipelinesDB = new(dbfakes.FakePipelinesDB)

	authValidator = new(authfakes.FakeValidator)
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

	fakeVolumeFactory = new(dbngfakes.FakeVolumeFactory)
	fakeContainerFactory = new(dbngfakes.FakeContainerFactory)

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
			authValidator,
			authValidator,
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

		pipelineDBFactory,
		teamDBFactory,
		dbTeamFactory,
		dbPipelineFactory,
		dbWorkerFactory,
		fakeVolumeFactory,
		fakeContainerFactory,
		dbBuildFactory,

		pipeDB,
		pipelinesDB,

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
