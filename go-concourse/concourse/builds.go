package concourse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) CreateBuild(plan atc.Plan) (atc.Build, error) {
	var build atc.Build

	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(plan)
	if err != nil {
		return build, fmt.Errorf("Unable to marshal plan: %s", err)
	}
	err = team.connection.Send(internal.Request{
		RequestName: atc.CreateBuild,
		Body:        buffer,
		Params: rata.Params{
			"team_name": team.Name(),
		},
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	}, &internal.Response{
		Result: &build,
	})

	return build, err
}

func (team *team) CreateJobBuild(pipelineName string, jobName string) (atc.Build, error) {
	params := rata.Params{
		"job_name":      jobName,
		"pipeline_name": pipelineName,
		"team_name":     team.name,
	}

	var build atc.Build
	err := team.connection.Send(internal.Request{
		RequestName: atc.CreateJobBuild,
		Params:      params,
	}, &internal.Response{
		Result: &build,
	})

	return build, err
}

func (team *team) JobBuild(pipelineName, jobName, buildName string) (atc.Build, bool, error) {
	params := rata.Params{
		"job_name":      jobName,
		"build_name":    buildName,
		"pipeline_name": pipelineName,
		"team_name":     team.name,
	}

	var build atc.Build
	err := team.connection.Send(internal.Request{
		RequestName: atc.GetJobBuild,
		Params:      params,
	}, &internal.Response{
		Result: &build,
	})

	switch err.(type) {
	case nil:
		return build, true, nil
	case internal.ResourceNotFoundError:
		return build, false, nil
	default:
		return build, false, err
	}
}

func (client *client) Build(buildID string) (atc.Build, bool, error) {
	params := rata.Params{
		"build_id": buildID,
	}

	var build atc.Build
	err := client.connection.Send(internal.Request{
		RequestName: atc.GetBuild,
		Params:      params,
	}, &internal.Response{
		Result: &build,
	})

	switch err.(type) {
	case nil:
		return build, true, nil
	case internal.ResourceNotFoundError:
		return build, false, nil
	default:
		return build, false, err
	}
}

func (client *client) Builds(page Page) ([]atc.Build, Pagination, error) {
	var builds []atc.Build

	headers := http.Header{}
	err := client.connection.Send(internal.Request{
		RequestName: atc.ListBuilds,
		Query:       page.QueryParams(),
	}, &internal.Response{
		Result:  &builds,
		Headers: &headers,
	})

	switch err.(type) {
	case nil:
		pagination, err := paginationFromHeaders(headers)
		if err != nil {
			return nil, Pagination{}, err
		}

		return builds, pagination, nil
	default:
		return nil, Pagination{}, err
	}
}

func (client *client) AbortBuild(buildID string) error {
	params := rata.Params{
		"build_id": buildID,
	}

	return client.connection.Send(internal.Request{
		RequestName: atc.AbortBuild,
		Params:      params,
	}, nil)
}

func (team *team) Builds(page Page) ([]atc.Build, Pagination, error) {
	var builds []atc.Build

	headers := http.Header{}

	params := rata.Params{
		"team_name": team.name,
	}

	err := team.connection.Send(internal.Request{
		RequestName: atc.ListTeamBuilds,
		Params:      params,
		Query:       page.QueryParams(),
	}, &internal.Response{
		Result:  &builds,
		Headers: &headers,
	})

	switch err.(type) {
	case nil:
		pagination, err := paginationFromHeaders(headers)
		if err != nil {
			return nil, Pagination{}, err
		}

		return builds, pagination, nil
	default:
		return nil, Pagination{}, err
	}
}

func (client *client) ListBuildArtifacts(buildID string) ([]atc.WorkerArtifact, error) {
	params := rata.Params{
		"build_id": buildID,
	}

	var artifacts []atc.WorkerArtifact

	err := client.connection.Send(internal.Request{
		RequestName: atc.ListBuildArtifacts,
		Params:      params,
	}, &internal.Response{
		Result: &artifacts,
	})

	return artifacts, err
}
