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

	"github.com/concourse/atc"
	"github.com/concourse/atc/api"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/dbng/dbngfakes"

	"github.com/concourse/atc/api/buildserver/buildserverfakes"
	"github.com/concourse/atc/api/containerserver/containerserverfakes"
	"github.com/concourse/atc/api/jobserver/jobserverfakes"
	"github.com/concourse/atc/api/pipes/pipesfakes"
	"github.com/concourse/atc/api/resourceserver/resourceserverfakes"
	"github.com/concourse/atc/api/teamserver/teamserverfakes"
	"github.com/concourse/atc/api/volumeserver/volumeserverfakes"
	"github.com/concourse/atc/auth/authfakes"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
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
	fakeTokenGenerator            *authfakes.FakeTokenGenerator
	providerFactory               *authfakes.FakeProviderFactory
	fakeEngine                    *enginefakes.FakeEngine
	fakeWorkerClient              *workerfakes.FakeClient
	teamServerDB                  *teamserverfakes.FakeTeamsDB
	volumesDB                     *volumeserverfakes.FakeVolumesDB
	containerDB                   *containerserverfakes.FakeContainerDB
	pipeDB                        *pipesfakes.FakePipeDB
	pipelineDBFactory             *dbfakes.FakePipelineDBFactory
	teamDBFactory                 *dbfakes.FakeTeamDBFactory
	dbTeamFactory                 *dbngfakes.FakeTeamFactory
	dbWorkerFactory               *dbngfakes.FakeWorkerFactory
	teamDB                        *dbfakes.FakeTeamDB
	pipelinesDB                   *dbfakes.FakePipelinesDB
	buildsDB                      *authfakes.FakeBuildsDB
	buildServerDB                 *buildserverfakes.FakeBuildsDB
	build                         *dbfakes.FakeBuild
	dbTeam                        *dbngfakes.FakeTeam
	fakeSchedulerFactory          *jobserverfakes.FakeSchedulerFactory
	fakeScannerFactory            *resourceserverfakes.FakeScannerFactory
	configValidationErrorMessages []string
	configValidationWarnings      []config.Warning
	peerAddr                      string
	drain                         chan struct{}
	expire                        time.Duration
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
	pipelineDBFactory = new(dbfakes.FakePipelineDBFactory)
	teamDBFactory = new(dbfakes.FakeTeamDBFactory)
	teamServerDB = new(teamserverfakes.FakeTeamsDB)
	teamDB = new(dbfakes.FakeTeamDB)
	teamDBFactory.GetTeamDBReturns(teamDB)
	buildServerDB = new(buildserverfakes.FakeBuildsDB)
	containerDB = new(containerserverfakes.FakeContainerDB)
	volumesDB = new(volumeserverfakes.FakeVolumesDB)
	pipeDB = new(pipesfakes.FakePipeDB)
	pipelinesDB = new(dbfakes.FakePipelinesDB)
	buildsDB = new(authfakes.FakeBuildsDB)

	dbTeamFactory = new(dbngfakes.FakeTeamFactory)
	dbTeam = new(dbngfakes.FakeTeam)
	dbTeamFactory.FindTeamReturns(dbTeam, true, nil)

	dbWorkerFactory = new(dbngfakes.FakeWorkerFactory)

	authValidator = new(authfakes.FakeValidator)
	userContextReader = new(authfakes.FakeUserContextReader)
	fakeTokenGenerator = new(authfakes.FakeTokenGenerator)
	providerFactory = new(authfakes.FakeProviderFactory)

	configValidationErrorMessages = []string{}
	configValidationWarnings = []config.Warning{}
	peerAddr = "127.0.0.1:1234"
	drain = make(chan struct{})

	fakeEngine = new(enginefakes.FakeEngine)
	fakeWorkerClient = new(workerfakes.FakeClient)

	fakeSchedulerFactory = new(jobserverfakes.FakeSchedulerFactory)
	fakeScannerFactory = new(resourceserverfakes.FakeScannerFactory)

	var err error

	cliDownloadsDir, err = ioutil.TempDir("", "cli-downloads")
	Expect(err).NotTo(HaveOccurred())

	constructedEventHandler = &fakeEventHandlerFactory{}

	logger = lagertest.NewTestLogger("callbacks")

	sink = lager.NewReconfigurableSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG), lager.DEBUG)
	logger.RegisterSink(sink)

	expire = 24 * time.Hour

	build = new(dbfakes.FakeBuild)

	checkPipelineAccessHandlerFactory := auth.NewCheckPipelineAccessHandlerFactory(pipelineDBFactory, teamDBFactory)

	checkBuildReadAccessHandlerFactory := auth.NewCheckBuildReadAccessHandlerFactory(buildsDB)

	checkBuildWriteAccessHandlerFactory := auth.NewCheckBuildWriteAccessHandlerFactory(buildsDB)

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

		fakeTokenGenerator,
		providerFactory,
		oAuthBaseURL,

		pipelineDBFactory,
		teamDBFactory,

		dbTeamFactory,
		dbWorkerFactory,

		teamServerDB,
		buildServerDB,
		containerDB,
		volumesDB,
		pipeDB,
		pipelinesDB,

		func(atc.Config) ([]config.Warning, []string) {
			return configValidationWarnings, configValidationErrorMessages
		},
		peerAddr,
		constructedEventHandler.Construct,
		drain,

		fakeEngine,
		fakeWorkerClient,

		fakeSchedulerFactory,
		fakeScannerFactory,

		sink,

		expire,

		cliDownloadsDir,
		"1.2.3",
	)
	Expect(err).NotTo(HaveOccurred())

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
