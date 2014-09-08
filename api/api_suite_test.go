package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc/api"
	buildfakes "github.com/concourse/atc/api/buildserver/fakes"
	jobfakes "github.com/concourse/atc/api/jobserver/fakes"
	"github.com/concourse/atc/builder/fakebuilder"
	"github.com/concourse/atc/logfanout"
	logfakes "github.com/concourse/atc/logfanout/fakes"
)

var (
	buildsDB     *buildfakes.FakeBuildsDB
	jobsDB       *jobfakes.FakeJobsDB
	logDB        *logfakes.FakeLogDB
	builder      *fakebuilder.FakeBuilder
	tracker      *logfanout.Tracker
	pingInterval time.Duration
	peerAddr     string

	server *httptest.Server
	client *http.Client
)

var _ = BeforeEach(func() {
	buildsDB = new(buildfakes.FakeBuildsDB)
	jobsDB = new(jobfakes.FakeJobsDB)
	logDB = new(logfakes.FakeLogDB)
	builder = new(fakebuilder.FakeBuilder)
	tracker = logfanout.NewTracker(logDB)
	pingInterval = 100 * time.Millisecond
	peerAddr = "127.0.0.1:1234"

	handler, err := api.NewHandler(
		lagertest.NewTestLogger("callbacks"),
		buildsDB,
		jobsDB,
		builder,
		tracker,
		pingInterval,
		peerAddr,
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
