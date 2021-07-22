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
