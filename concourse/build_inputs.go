package concourse

import (
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
