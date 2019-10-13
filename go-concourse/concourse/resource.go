package concourse

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
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

func (team *team) ListResources(pipelineName string) ([]atc.Resource, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"team_name":     team.name,
	}

	var resources []atc.Resource
	err := team.connection.Send(internal.Request{
		RequestName: atc.ListResources,
		Params:      params,
	}, &internal.Response{
		Result: &resources,
	})

	return resources, err
}

func (team *team) ClearResourceCache(pipelineName string, resourceName string, version atc.Version) ([]string, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"resource_name": resourceName,
		"team_name":     team.name,
	}

	requestName := atc.ClearResourceCaches

	if version != nil {
		selectedVersion, _ := version["ref"]
		params["resource_config_version_id"] = selectedVersion
		requestName = atc.ClearResourceCache
	}

	response := make([]string, 0)
	err := team.connection.Send(internal.Request{
		RequestName: requestName,
		Params:      params,
	}, &internal.Response{
		Result: &response,
	})
	return response, err
}
