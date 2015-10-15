package atcclient

import "github.com/concourse/atc"

func (buildHandler AtcHandler) Job(pipelineName, jobName string) (atc.Job, error) {
	if pipelineName == "" {
		pipelineName = atc.DefaultPipelineName
	}
	params := map[string]string{"pipeline_name": pipelineName, "job_name": jobName}
	var job atc.Job
	err := buildHandler.client.MakeRequest(&job, atc.GetJob, params, nil)
	return job, err
}
