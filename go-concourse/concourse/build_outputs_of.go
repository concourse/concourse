package concourse

import (
	"strconv"

	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) BuildsWithVersionAsOutput(pipelineRef atc.PipelineRef, resourceName string, resourceVersionID int) ([]atc.Build, bool, error) {
	params := rata.Params{
		"team_name":                  team.Name(),
		"pipeline_name":              pipelineRef.Name,
		"resource_name":              resourceName,
		"resource_config_version_id": strconv.Itoa(resourceVersionID),
	}

	var builds []atc.Build
	err := team.connection.Send(internal.Request{
		RequestName: atc.ListBuildsWithVersionAsOutput,
		Params:      params,
		Query:       pipelineRef.QueryParams(),
	}, &internal.Response{
		Result: &builds,
	})

	switch err.(type) {
	case nil:
		return builds, true, nil
	case internal.ResourceNotFoundError:
		return builds, false, nil
	default:
		return builds, false, err
	}
}
