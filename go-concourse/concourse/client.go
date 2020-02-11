package concourse

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
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
	ListTeams() ([]atc.Team, error)
	FindTeam(teamName string) (Team, error)
	Team(teamName string) Team
	UserInfo() (map[string]interface{}, error)
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

func (client *client) FindTeam(teamName string) (Team, error) {
	var atcTeam atc.Team
	resp, err := client.httpAgent.Send(internal.Request{
		RequestName: atc.GetTeam,
		Params:      rata.Params{"team_name": teamName},
	})

	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		err = json.NewDecoder(resp.Body).Decode(&atcTeam)
		if err != nil {
			return nil, err
		}
		return &team{
			name:       atcTeam.Name,
			connection: client.connection,
			httpAgent:  client.httpAgent,
			auth:       atcTeam.Auth,
		}, nil
	case http.StatusForbidden:
		return nil, fmt.Errorf("you do not have a role on team '%s'", teamName)
	case http.StatusNotFound:
		return nil, fmt.Errorf("team '%s' does not exist", teamName)
	default:
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, internal.UnexpectedResponseError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}


}
