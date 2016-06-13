package concourse

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
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

func (team *team) DeletePipeline(pipelineName string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"team_name":     team.name,
	}

	err := team.connection.Send(internal.Request{
		RequestName: atc.DeletePipeline,
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

func (team *team) PausePipeline(pipelineName string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"team_name":     team.name,
	}
	err := team.connection.Send(internal.Request{
		RequestName: atc.PausePipeline,
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

func (team *team) UnpausePipeline(pipelineName string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"team_name":     team.name,
	}

	err := team.connection.Send(internal.Request{
		RequestName: atc.UnpausePipeline,
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

	jsonBytes, err := json.Marshal(struct {
		Name string `json:"name"`
	}{Name: name})
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
