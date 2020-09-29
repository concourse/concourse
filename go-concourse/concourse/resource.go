package concourse

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
)

func (team *team) Resource(pipelineRef atc.PipelineRef, resourceName string) (atc.Resource, bool, error) {
	params := map[string]string{
		"pipeline_name": pipelineRef.Name,
		"resource_name": resourceName,
		"team_name":     team.Name(),
	}

	var resource atc.Resource
	err := team.connection.Send(internal.Request{
		RequestName: atc.GetResource,
		Params:      params,
		Query:       pipelineRef.QueryParams(),
	}, &internal.Response{
		Result: &resource,
	})
	switch err.(type) {
	case nil:
		return resource, true, nil
	case internal.ResourceNotFoundError:
		return resource, false, nil
	default:
		return resource, false, err
	}
}

func (team *team) ListResources(pipelineRef atc.PipelineRef) ([]atc.Resource, error) {
	params := map[string]string{
		"pipeline_name": pipelineRef.Name,
		"team_name":     team.Name(),
	}

	var resources []atc.Resource
	err := team.connection.Send(internal.Request{
		RequestName: atc.ListResources,
		Params:      params,
		Query:       pipelineRef.QueryParams(),
	}, &internal.Response{
		Result: &resources,
	})

	return resources, err
}
