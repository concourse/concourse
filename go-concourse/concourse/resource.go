package concourse

import (
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) Resource(pipelineName string, resourceName string) (atc.Resource, bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"resource_name": resourceName,
		"team_name":     team.name,
	}

	var resource atc.Resource
	err := team.connection.Send(internal.Request{
		RequestName: atc.GetResource,
		Params:      params,
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

func (team *team) PauseResource(pipelineName string, resourceName string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"resource_name": resourceName,
		"team_name":     team.name,
	}

	err := team.connection.Send(internal.Request{
		RequestName: atc.PauseResource,
		Params:      params,
	}, &internal.Response{})

	switch err.(type) {
	case nil:
		return true, nil
	case internal.ResourceNotFoundError:
		return false, nil
	default:
		return false, err
	}
}

func (team *team) UnpauseResource(pipelineName string, resourceName string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"resource_name": resourceName,
		"team_name":     team.name,
	}
	err := team.connection.Send(internal.Request{
		RequestName: atc.UnpauseResource,
		Params:      params,
	}, &internal.Response{})

	switch err.(type) {
	case nil:
		return true, nil
	case internal.ResourceNotFoundError:
		return false, nil
	default:
		return false, err
	}
}
