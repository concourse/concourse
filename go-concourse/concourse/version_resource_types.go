package concourse

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) VersionedResourceTypes(pipelineRef atc.PipelineRef) (atc.VersionedResourceTypes, bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
		"team_name":     team.Name(),
	}

	var versionedResourceTypes atc.VersionedResourceTypes
	err := team.connection.Send(internal.Request{
		RequestName: atc.ListResourceTypes,
		Params:      params,
		Query:       pipelineRef.QueryParams(),
	}, &internal.Response{
		Result: &versionedResourceTypes,
	})

	switch err.(type) {
	case nil:
		return versionedResourceTypes, true, nil
	case internal.ResourceNotFoundError:
		return versionedResourceTypes, false, nil
	default:
		return versionedResourceTypes, false, err
	}
}
