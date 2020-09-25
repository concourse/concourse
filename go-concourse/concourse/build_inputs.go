package concourse

import (
	"strconv"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) BuildInputsForJob(pipelineName string, jobName string) ([]atc.BuildInput, bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"job_name":      jobName,
		"team_name":     team.Name(),
	}

	var buildInputs []atc.BuildInput
	err := team.connection.Send(internal.Request{
		RequestName: atc.ListJobInputs,
		Params:      params,
	}, &internal.Response{
		Result: &buildInputs,
	})

	switch err.(type) {
	case nil:
		return buildInputs, true, nil
	case internal.ResourceNotFoundError:
		return buildInputs, false, nil
	default:
		return buildInputs, false, err
	}
}

func (team *team) BuildsWithVersionAsInput(pipelineName string, resourceName string, resourceVersionID int) ([]atc.Build, bool, error) {
	params := rata.Params{
		"pipeline_name":              pipelineName,
		"resource_name":              resourceName,
		"resource_config_version_id": strconv.Itoa(resourceVersionID),
		"team_name":                  team.Name(),
	}

	var builds []atc.Build
	err := team.connection.Send(internal.Request{
		RequestName: atc.ListBuildsWithVersionAsInput,
		Params:      params,
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
