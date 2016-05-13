package concourse

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (client *client) Pipeline(pipelineName string) (atc.Pipeline, bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"team_name":     atc.DefaultTeamName,
	}

	var pipeline atc.Pipeline
	err := client.connection.Send(internal.Request{
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

func (client *client) ListPipelines() ([]atc.Pipeline, error) {
	params := rata.Params{
		"team_name": atc.DefaultTeamName,
	}

	var pipelines []atc.Pipeline
	err := client.connection.Send(internal.Request{
		RequestName: atc.ListPipelines,
		Params:      params,
	}, &internal.Response{
		Result: &pipelines,
	})

	return pipelines, err
}

func (client *client) DeletePipeline(pipelineName string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"team_name":     atc.DefaultTeamName,
	}

	err := client.connection.Send(internal.Request{
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

func (client *client) PausePipeline(pipelineName string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"team_name":     atc.DefaultTeamName,
	}
	err := client.connection.Send(internal.Request{
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

func (client *client) UnpausePipeline(pipelineName string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"team_name":     atc.DefaultTeamName,
	}

	err := client.connection.Send(internal.Request{
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

func (client *client) RenamePipeline(pipelineName, name string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"team_name":     atc.DefaultTeamName,
	}

	jsonBytes, err := json.Marshal(struct {
		Name string `json:"name"`
	}{Name: name})
	if err != nil {
		return false, err
	}

	err = client.connection.Send(internal.Request{
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
