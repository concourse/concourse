package concourse

import (
	"strconv"

	"github.com/concourse/atc"
)

func (client *client) Job(pipelineName, jobName string) (atc.Job, bool, error) {
	if pipelineName == "" {
		return atc.Job{}, false, NameRequiredError("pipeline")
	}

	params := map[string]string{"pipeline_name": pipelineName, "job_name": jobName}
	var job atc.Job
	err := client.connection.Send(Request{
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

func (client *client) JobBuilds(pipelineName string, jobName string, since int, until int, limit int) ([]atc.Build, bool, error) {
	params := map[string]string{"pipeline_name": pipelineName, "job_name": jobName}
	queryParams := map[string]string{}

	if until > 0 {
		queryParams["until"] = strconv.Itoa(until)
	} else if since > 0 {
		queryParams["since"] = strconv.Itoa(since)
	}

	if limit > 0 {
		queryParams["limit"] = strconv.Itoa(limit)
	}

	var builds []atc.Build
	err := client.connection.Send(Request{
		RequestName: atc.ListJobBuilds,
		Params:      params,
		Queries:     queryParams,
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
