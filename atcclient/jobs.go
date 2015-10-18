package atcclient

import "github.com/concourse/atc"

func (handler AtcHandler) Job(pipelineName, jobName string) (atc.Job, error) {
	if pipelineName == "" {
		pipelineName = atc.DefaultPipelineName
	}
	params := map[string]string{"pipeline_name": pipelineName, "job_name": jobName}
	var job atc.Job
	err := handler.client.MakeRequest(&job, atc.GetJob, params, nil, nil)
	return job, err
}
