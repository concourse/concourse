package concourse

import (
	"strconv"

	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
)

func (client *client) BuildResources(buildID int) (atc.BuildInputsOutputs, bool, error) {
	params := rata.Params{"build_id": strconv.Itoa(buildID)}

	var buildInputsOutputs atc.BuildInputsOutputs
	err := client.connection.Send(Request{
		RequestName: atc.BuildResources,
		Params:      params,
	}, &Response{
		Result: &buildInputsOutputs,
	})

	switch err.(type) {
	case nil:
		return buildInputsOutputs, true, nil
	case ResourceNotFoundError:
		return buildInputsOutputs, false, nil
	default:
		return buildInputsOutputs, false, err
	}
}
