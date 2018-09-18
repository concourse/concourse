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

func (team *team) Pipeline(pipelineName string) (atc.Pipeline, bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"team_name":     team.name,
	}

	var pipeline atc.Pipeline
	err := team.connection.Send(internal.Request{
		RequestName: atc.GetPipeline,
		Params:      params,
	}, &internal.Response{
		Result: &pipeline,
	})

	switch err.(type) {
	case nil:
		return pipeline, true, nil
	case internal.ResourceNotFoundError:
		return atc.Pipeline{}, false, nil
	default:
		return atc.Pipeline{}, false, err
	}
}

func (team *team) OrderingPipelines(pipelines []string) error {
	params := rata.Params{
		"team_name": team.name,
	}

	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(pipelines)
	if err != nil {
		return fmt.Errorf("Unable to marshal pipeline names: %s", err)
	}

	return team.connection.Send(internal.Request{
		RequestName: atc.OrderPipelines,
		Params:      params,
		Body:        buffer,
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	}, &internal.Response{})
}

func (team *team) ListPipelines() ([]atc.Pipeline, error) {
	params := rata.Params{
		"team_name": team.name,
	}

	var pipelines []atc.Pipeline
	err := team.connection.Send(internal.Request{
		RequestName: atc.ListPipelines,
		Params:      params,
	}, &internal.Response{
		Result: &pipelines,
	})

	return pipelines, err
}

func (client *client) ListPipelines() ([]atc.Pipeline, error) {
	var pipelines []atc.Pipeline
	err := client.connection.Send(internal.Request{
		RequestName: atc.ListAllPipelines,
	}, &internal.Response{
		Result: &pipelines,
	})

	return pipelines, err
}

func (team *team) CreatePipelineBuild(pipelineName string, plan atc.Plan) (atc.Build, error) {
	var build atc.Build

	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(plan)
	if err != nil {
		return build, fmt.Errorf("Unable to marshal plan: %s", err)
	}

	err = team.connection.Send(internal.Request{
		RequestName: atc.CreatePipelineBuild,
		Body:        buffer,
		Params: rata.Params{
			"team_name":     team.name,
			"pipeline_name": pipelineName,
		},
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	}, &internal.Response{
		Result: &build,
	})

	return build, err
}
func (team *team) DeletePipeline(pipelineName string) (bool, error) {
	return team.managePipeline(pipelineName, atc.DeletePipeline)
}

func (team *team) PausePipeline(pipelineName string) (bool, error) {

	return team.managePipeline(pipelineName, atc.PausePipeline)
}

func (team *team) UnpausePipeline(pipelineName string) (bool, error) {
	return team.managePipeline(pipelineName, atc.UnpausePipeline)
}

func (team *team) ExposePipeline(pipelineName string) (bool, error) {
	return team.managePipeline(pipelineName, atc.ExposePipeline)
}

func (team *team) HidePipeline(pipelineName string) (bool, error) {
	return team.managePipeline(pipelineName, atc.HidePipeline)
}

func (team *team) managePipeline(pipelineName string, endpoint string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"team_name":     team.name,
	}
	err := team.connection.Send(internal.Request{
		RequestName: endpoint,
		Params:      params,
	}, nil)

	switch err.(type) {
	case nil:
		return true, nil
	case internal.ResourceNotFoundError:
		return false, nil
	default:
		return false, err
	}
}

func (team *team) RenamePipeline(pipelineName, name string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"team_name":     team.name,
	}

	jsonBytes, err := json.Marshal(atc.RenameRequest{NewName: name})
	if err != nil {
		return false, err
	}

	err = team.connection.Send(internal.Request{
		RequestName: atc.RenamePipeline,
		Params:      params,
		Body:        bytes.NewBuffer(jsonBytes),
		Header:      http.Header{"Content-Type": []string{"application/json"}},
	}, nil)
	switch err.(type) {
	case nil:
		return true, nil
	case internal.ResourceNotFoundError:
		return false, nil
	default:
		return false, err
	}
}

func (team *team) PipelineBuilds(pipelineName string, page Page) ([]atc.Build, Pagination, bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"team_name":     team.name,
	}

	var builds []atc.Build

	headers := http.Header{}
	err := team.connection.Send(internal.Request{
		RequestName: atc.ListPipelineBuilds,
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
			return builds, Pagination{}, false, err
		}

		return builds, pagination, true, nil
	case internal.ResourceNotFoundError:
		return builds, Pagination{}, false, nil
	default:
		return builds, Pagination{}, false, err
	}
}
