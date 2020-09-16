package concourse

import (
	"io"
	"net/http"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
)

//go:generate counterfeiter . Client

type Client interface {
	URL() string
	HTTPClient() *http.Client
	Builds(Page) ([]atc.Build, Pagination, error)
	Build(buildID string) (atc.Build, bool, error)
	BuildEvents(buildID string) (Events, error)
	BuildResources(buildID int) (atc.BuildInputsOutputs, bool, error)
	ListBuildArtifacts(buildID string) ([]atc.WorkerArtifact, error)
	AbortBuild(buildID string) error
	BuildPlan(buildID int) (atc.PublicBuildPlan, bool, error)
	SaveWorker(atc.Worker, *time.Duration) (*atc.Worker, error)
	ListWorkers() ([]atc.Worker, error)
	PruneWorker(workerName string) error
	LandWorker(workerName string) error
	GetInfo() (atc.Info, error)
	GetCLIReader(arch, platform string) (io.ReadCloser, http.Header, error)
	ListPipelines() ([]atc.Pipeline, error)
	ListAllJobs() ([]atc.Job, error)
	ListTeams() ([]atc.Team, error)
	FindTeam(teamName string) (Team, error)
	Team(teamName string) Team
	UserInfo() (atc.UserInfo, error)
	ListActiveUsersSince(since time.Time) ([]atc.User, error)
	Check(checkID string) (atc.Check, bool, error)
}

type client struct {
	connection internal.Connection //Deprecated
	httpAgent  internal.HTTPAgent
}

func NewClient(apiURL string, httpClient *http.Client, tracing bool) Client {
	return &client{
		connection: internal.NewConnection(apiURL, httpClient, tracing),
		httpAgent:  internal.NewHTTPAgent(apiURL, httpClient, tracing),
	}
}

func (client *client) URL() string {
	return client.connection.URL()
}

func (client *client) HTTPClient() *http.Client {
	return client.connection.HTTPClient()
}
