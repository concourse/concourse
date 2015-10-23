package atcclient

import "github.com/concourse/atc"

func (handler AtcHandler) Job(pipelineName, jobName string) (atc.Job, bool, error) {
	if pipelineName == "" {
		return atc.Job{}, false, NameRequiredError("pipeline")
	}

	params := map[string]string{"pipeline_name": pipelineName, "job_name": jobName}
	var job atc.Job
	err := handler.client.Send(Request{
		RequestName: atc.GetJob,
		Params:      params,
	}, &Response{
		Result: &job,
	})

	switch err.(type) {
	case nil:
		return job, true, nil
	case ResourceNotFoundError:
		return job, false, nil
	default:
		return job, false, err
	}
}
