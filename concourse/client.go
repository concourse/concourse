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
	BuildPlan(buildID int) (atc.PublicBuildPlan, bool, error)
	SendInputToBuildPlan(buildID int, planID atc.PlanID, src io.Reader) (bool, error)
	ReadOutputFromBuildPlan(buildID int, planID atc.PlanID) (io.ReadCloser, bool, error)
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
