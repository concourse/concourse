package concourse

import (
	"io"
	"net/http"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
)

//go:generate counterfeiter . Client

type Client interface {
	URL() string
	HTTPClient() *http.Client

	Builds(Page) ([]atc.Build, Pagination, error)
	Build(buildID string) (atc.Build, bool, error)
	BuildEvents(buildID string) (Events, error)
	BuildResources(buildID int) (atc.BuildInputsOutputs, bool, error)
	AbortBuild(buildID string) error
	CreateBuild(plan atc.Plan) (atc.Build, error)
	BuildPlan(buildID int) (atc.PublicBuildPlan, bool, error)
	CreatePipe() (atc.Pipe, error)
	ListContainers(queryList map[string]string) ([]atc.Container, error)
	ListVolumes() ([]atc.Volume, error)
	SaveWorker(atc.Worker, *time.Duration) (*atc.Worker, error)
	ListWorkers() ([]atc.Worker, error)
	PruneWorker(workerName string) error
	GetInfo() (atc.Info, error)
	GetCLIReader(arch, platform string) (io.ReadCloser, http.Header, error)
	ListPipelines() ([]atc.Pipeline, error)
	ListTeams() ([]atc.Team, error)

	Team(teamName string) Team
}

type client struct {
	connection internal.Connection
}

func NewClient(apiURL string, httpClient *http.Client, tracing bool) Client {
	return &client{
		connection: internal.NewConnection(apiURL, httpClient, tracing),
	}
}

func (client *client) URL() string {
	return client.connection.URL()
}

func (client *client) HTTPClient() *http.Client {
	return client.connection.HTTPClient()
}
