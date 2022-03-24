package concourse

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) ResourceTypes(pipelineRef atc.PipelineRef) (atc.ResourceTypes, bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
		"team_name":     team.Name(),
	}

	var resourceTypes atc.ResourceTypes
	err := team.connection.Send(internal.Request{
		RequestName: atc.ListResourceTypes,
		Params:      params,
		Query:       pipelineRef.QueryParams(),
	}, &internal.Response{
		Result: &resourceTypes,
	})

	switch err.(type) {
	case nil:
		return resourceTypes, true, nil
	case internal.ResourceNotFoundError:
		return resourceTypes, false, nil
	default:
		return resourceTypes, false, err
	}
}

func (team *team) ListSharedForResourceType(pipelineRef atc.PipelineRef, resourceTypeName string) (atc.ResourcesAndTypes, bool, error) {
	params := rata.Params{
		"pipeline_name":      pipelineRef.Name,
		"resource_type_name": resourceTypeName,
		"team_name":          team.Name(),
	}

	var shared atc.ResourcesAndTypes
	err := team.connection.Send(internal.Request{
		RequestName: atc.ListSharedForResourceType,
		Params:      params,
		Query:       pipelineRef.QueryParams(),
	}, &internal.Response{
		Result: &shared,
	})
	switch err.(type) {
	case nil:
		return shared, true, nil
	case internal.ResourceNotFoundError:
		return shared, false, nil
	default:
		return shared, false, err
	}
}
