package atcclient

import "github.com/concourse/atc"

func (handler AtcHandler) BuildInputsForJob(pipelineName string, jobName string) ([]atc.BuildInput, error) {
	params := map[string]string{"pipeline_name": pipelineName, "job_name": jobName}
	var buildInputs []atc.BuildInput
	err := handler.client.MakeRequest(&buildInputs, atc.ListJobInputs, params, nil, nil)
	return buildInputs, err
}
