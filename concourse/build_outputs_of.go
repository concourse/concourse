package concourse

import (
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (client *client) BuildsWithVersionAsOutput(pipelineName string, resourceName string, resourceVersionID int) ([]atc.Build, bool, error) {
	params := rata.Params{
		"pipeline_name":       pipelineName,
		"resource_name":       resourceName,
		"resource_version_id": strconv.Itoa(resourceVersionID),
	}

	var builds []atc.Build
	err := client.connection.Send(internal.Request{
		RequestName: atc.ListBuildsWithVersionAsOutput,
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
