package concourse

import (
	"strconv"

	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (client *client) BuildResources(buildID int) (atc.BuildInputsOutputs, bool, error) {
	params := rata.Params{
		"build_id": strconv.Itoa(buildID),
	}

	var buildInputsOutputs atc.BuildInputsOutputs
	err := client.connection.Send(internal.Request{
		RequestName: atc.BuildResources,
		Params:      params,
	}, &internal.Response{
		Result: &buildInputsOutputs,
	})

	switch err.(type) {
	case nil:
		return buildInputsOutputs, true, nil
	case internal.ResourceNotFoundError:
		return buildInputsOutputs, false, nil
	default:
		return buildInputsOutputs, false, err
	}
}
