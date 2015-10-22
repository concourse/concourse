package atcclient

import "github.com/concourse/atc"

func (handler AtcHandler) BuildInputsForJob(pipelineName string, jobName string) ([]atc.BuildInput, bool, error) {
	params := map[string]string{"pipeline_name": pipelineName, "job_name": jobName}

	var buildInputs []atc.BuildInput
	err := handler.client.Send(Request{
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
