package concourse

import (
	"strconv"

	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
)

func (client *client) BuildInputsForJob(pipelineName string, jobName string) ([]atc.BuildInput, bool, error) {
	params := rata.Params{"pipeline_name": pipelineName, "job_name": jobName}

	var buildInputs []atc.BuildInput
	err := client.connection.Send(Request{
		RequestName: atc.ListJobInputs,
		Params:      params,
	}, &Response{
		Result: &buildInputs,
	})

	switch err.(type) {
	case nil:
		return buildInputs, true, nil
	case ResourceNotFoundError:
		return buildInputs, false, nil
	default:
		return buildInputs, false, err
	}
}

func (client *client) BuildsWithVersionAsInput(pipelineName string, resourceName string, resourceVersionID int) ([]atc.Build, bool, error) {
	params := rata.Params{
		"pipeline_name":       pipelineName,
		"resource_name":       resourceName,
		"resource_version_id": strconv.Itoa(resourceVersionID),
	}

	var builds []atc.Build
	err := client.connection.Send(Request{
		RequestName: atc.ListBuildsWithVersionAsInput,
		Params:      params,
	}, &Response{
		Result: &builds,
	})

	switch err.(type) {
	case nil:
		return builds, true, nil
	case ResourceNotFoundError:
		return builds, false, nil
	default:
		return builds, false, err
	}
}
