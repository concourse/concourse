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
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/builder/fakebuilder"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	"github.com/concourse/atc/event"
)

var (
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
	censor  event.Censor

	lock sync.Mutex
}

func (f *fakeEventHandlerFactory) Construct(
	db event.BuildsDB,
	buildID int,
	censor event.Censor,
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
	configDBWithDefaults := db.ConfigDBWithDefaults{configDB}

	configValidationErr = nil
	builder = new(fakebuilder.FakeBuilder)
	pingInterval = 100 * time.Millisecond
	peerAddr = "127.0.0.1:1234"
	drain = make(chan struct{})

	constructedEventHandler = &fakeEventHandlerFactory{}

	handler, err := api.NewHandler(
		lagertest.NewTestLogger("callbacks"),
		auth.NoopValidator{},

		buildsDB,
		configDBWithDefaults,

		configDBWithDefaults,

		jobsDB,
		configDBWithDefaults,

		configDBWithDefaults,

		func(atc.Config) error { return configValidationErr },
		builder,
		pingInterval,
		peerAddr,
		constructedEventHandler.Construct,
		drain,
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
