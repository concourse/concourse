package api_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api"
	buildfakes "github.com/concourse/atc/api/buildserver/fakes"
	jobfakes "github.com/concourse/atc/api/jobserver/fakes"
	authfakes "github.com/concourse/atc/auth/fakes"
	"github.com/concourse/atc/builder/fakebuilder"
	dbfakes "github.com/concourse/atc/db/fakes"
	"github.com/concourse/atc/engine"
	enginefakes "github.com/concourse/atc/engine/fakes"
	"github.com/concourse/atc/event"
)

var (
	authValidator       *authfakes.FakeValidator
	fakeEngine          *enginefakes.FakeEngine
	buildsDB            *buildfakes.FakeBuildsDB
	jobsDB              *jobfakes.FakeJobsDB
	configDB            *dbfakes.FakeConfigDB
	configValidationErr error
	builder             *fakebuilder.FakeBuilder
	pingInterval        time.Duration
	peerAddr            string
	drain               chan struct{}

	constructedEventHandler *fakeEventHandlerFactory

	server *httptest.Server
	client *http.Client
)

type fakeEventHandlerFactory struct {
	db      event.BuildsDB
	buildID int
	engine  engine.Engine
	censor  event.Censor

	lock sync.Mutex
}

func (f *fakeEventHandlerFactory) Construct(
	db event.BuildsDB,
	buildID int,
	engine engine.Engine,
	censor event.Censor,
) http.Handler {
	f.lock.Lock()
	f.db = db
	f.buildID = buildID
	f.engine = engine
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

	authValidator = new(authfakes.FakeValidator)
	configValidationErr = nil
	builder = new(fakebuilder.FakeBuilder)
	pingInterval = 100 * time.Millisecond
	peerAddr = "127.0.0.1:1234"
	drain = make(chan struct{})

	fakeEngine = new(enginefakes.FakeEngine)

	constructedEventHandler = &fakeEventHandlerFactory{}

	handler, err := api.NewHandler(
		lagertest.NewTestLogger("callbacks"),
		authValidator,

		buildsDB,
		configDB,

		configDB,

		jobsDB,
		configDB,

		configDB,

		func(atc.Config) error { return configValidationErr },
		builder,
		pingInterval,
		peerAddr,
		constructedEventHandler.Construct,
		drain,

		fakeEngine,
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
