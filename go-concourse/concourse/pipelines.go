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

func (team *team) Pipeline(pipelineRef atc.PipelineRef) (atc.Pipeline, bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
		"team_name":     team.Name(),
	}

	var pipeline atc.Pipeline
	err := team.connection.Send(internal.Request{
		RequestName: atc.GetPipeline,
		Params:      params,
		Query:       pipelineRef.QueryParams(),
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

func (team *team) OrderingPipelines(pipelineRefs atc.OrderPipelinesRequest) error {
	params := rata.Params{
		"team_name": team.Name(),
	}

	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(pipelineRefs)
	if err != nil {
		return fmt.Errorf("Unable to marshal pipeline refs: %s", err)
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
		"team_name": team.Name(),
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

func (team *team) CreatePipelineBuild(pipelineRef atc.PipelineRef, plan atc.Plan) (atc.Build, error) {
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
			"team_name":     team.Name(),
			"pipeline_name": pipelineRef.Name,
		},
		Query: pipelineRef.QueryParams(),
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	}, &internal.Response{
		Result: &build,
	})

	return build, err
}
func (team *team) DeletePipeline(pipelineRef atc.PipelineRef) (bool, error) {
	return team.managePipeline(pipelineRef, atc.DeletePipeline)
}

func (team *team) PausePipeline(pipelineRef atc.PipelineRef) (bool, error) {
	return team.managePipeline(pipelineRef, atc.PausePipeline)
}

func (team *team) ArchivePipeline(pipelineRef atc.PipelineRef) (bool, error) {
	return team.managePipeline(pipelineRef, atc.ArchivePipeline)
}

func (team *team) UnpausePipeline(pipelineRef atc.PipelineRef) (bool, error) {
	return team.managePipeline(pipelineRef, atc.UnpausePipeline)
}

func (team *team) ExposePipeline(pipelineRef atc.PipelineRef) (bool, error) {
	return team.managePipeline(pipelineRef, atc.ExposePipeline)
}

func (team *team) HidePipeline(pipelineRef atc.PipelineRef) (bool, error) {
	return team.managePipeline(pipelineRef, atc.HidePipeline)
}

func (team *team) managePipeline(pipelineRef atc.PipelineRef, endpoint string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
		"team_name":     team.Name(),
	}
	err := team.connection.Send(internal.Request{
		RequestName: endpoint,
		Params:      params,
		Query:       pipelineRef.QueryParams(),
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

func (team *team) RenamePipeline(pipelineRef atc.PipelineRef, name string) (bool, []ConfigWarning, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
		"team_name":     team.Name(),
	}

	jsonBytes, err := json.Marshal(atc.RenameRequest{NewName: name})
	if err != nil {
		return false, []ConfigWarning{}, err
	}

	var response setConfigResponse
	err = team.connection.Send(internal.Request{
		RequestName: atc.RenamePipeline,
		Params:      params,
		Query:       pipelineRef.QueryParams(),
		Body:        bytes.NewBuffer(jsonBytes),
		Header:      http.Header{"Content-Type": []string{"application/json"}},
	}, &internal.Response{
		Result: &response,
	})

	switch err.(type) {
	case nil:
		return true, response.Warnings, nil
	case internal.ResourceNotFoundError:
		return false, []ConfigWarning{}, nil
	default:
		return false, []ConfigWarning{}, err
	}
}

func (team *team) PipelineBuilds(pipelineRef atc.PipelineRef, page Page) ([]atc.Build, Pagination, bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
		"team_name":     team.Name(),
	}

	var builds []atc.Build

	headers := http.Header{}
	err := team.connection.Send(internal.Request{
		RequestName: atc.ListPipelineBuilds,
		Params:      params,
		Query:       merge(page.QueryParams(), pipelineRef.QueryParams()),
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
