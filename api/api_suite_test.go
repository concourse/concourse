package api_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api"
	"github.com/concourse/atc/api/buildserver"
	buildfakes "github.com/concourse/atc/api/buildserver/fakes"
	jobfakes "github.com/concourse/atc/api/jobserver/fakes"
	resourcefakes "github.com/concourse/atc/api/resourceserver/fakes"
	workerfakes "github.com/concourse/atc/api/workerserver/fakes"
	authfakes "github.com/concourse/atc/auth/fakes"
	dbfakes "github.com/concourse/atc/db/fakes"
	enginefakes "github.com/concourse/atc/engine/fakes"
)

var (
	sink *lager.ReconfigurableSink

	authValidator       *authfakes.FakeValidator
	fakeEngine          *enginefakes.FakeEngine
	buildsDB            *buildfakes.FakeBuildsDB
	jobsDB              *jobfakes.FakeJobsDB
	configDB            *dbfakes.FakeConfigDB
	workerDB            *workerfakes.FakeWorkerDB
	resourceDB          *resourcefakes.FakeResourceDB
	configValidationErr error
	pingInterval        time.Duration
	peerAddr            string
	drain               chan struct{}

	constructedEventHandler *fakeEventHandlerFactory

	server *httptest.Server
	client *http.Client
)

type fakeEventHandlerFactory struct {
	db      buildserver.BuildsDB
	buildID int
	censor  bool

	lock sync.Mutex
}

func (f *fakeEventHandlerFactory) Construct(
	db buildserver.BuildsDB,
	buildID int,
	censor bool,
) http.Handler {
	f.lock.Lock()
	f.db = db
	f.buildID = buildID
	f.censor = censor
	f.lock.Unlock()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake event handler factory was here"))
	})
}

var _ = BeforeEach(func() {
	buildsDB = new(buildfakes.FakeBuildsDB)
	jobsDB = new(jobfakes.FakeJobsDB)
	configDB = new(dbfakes.FakeConfigDB)
	workerDB = new(workerfakes.FakeWorkerDB)
	resourceDB = new(resourcefakes.FakeResourceDB)

	authValidator = new(authfakes.FakeValidator)
	configValidationErr = nil
	pingInterval = 100 * time.Millisecond
	peerAddr = "127.0.0.1:1234"
	drain = make(chan struct{})

	fakeEngine = new(enginefakes.FakeEngine)

	constructedEventHandler = &fakeEventHandlerFactory{}

	logger := lagertest.NewTestLogger("callbacks")

	sink = lager.NewReconfigurableSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG), lager.DEBUG)
	logger.RegisterSink(sink)

	handler, err := api.NewHandler(
		logger,
		authValidator,

		buildsDB,
		configDB,

		configDB,

		jobsDB,
		configDB,

		resourceDB,
		configDB,

		workerDB,

		func(atc.Config) error { return configValidationErr },
		pingInterval,
		peerAddr,
		constructedEventHandler.Construct,
		drain,

		fakeEngine,

		sink,
	)
	Ω(err).ShouldNot(HaveOccurred())

	server = httptest.NewServer(handler)

	client = &http.Client{
		Transport: &http.Transport{},
	}
})

var _ = AfterEach(func() {
	server.Close()
})

func TestApi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Api Suite")
}
