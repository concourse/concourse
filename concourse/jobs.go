package concourse

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (client *client) Job(pipelineName, jobName string) (atc.Job, bool, error) {
	if pipelineName == "" {
		return atc.Job{}, false, NameRequiredError("pipeline")
	}

	params := rata.Params{"pipeline_name": pipelineName, "job_name": jobName}
	var job atc.Job
	err := client.connection.Send(internal.Request{
		RequestName: atc.GetJob,
		Params:      params,
	}, &internal.Response{
		Result: &job,
	})
	switch err.(type) {
	case nil:
		return job, true, nil
	case internal.ResourceNotFoundError:
		return job, false, nil
	default:
		return job, false, err
	}
}

func (client *client) JobBuilds(pipelineName string, jobName string, page Page) ([]atc.Build, Pagination, bool, error) {
	params := rata.Params{"pipeline_name": pipelineName, "job_name": jobName}
	var builds []atc.Build

	headers := http.Header{}
	err := client.connection.Send(internal.Request{
		RequestName: atc.ListJobBuilds,
		Params:      params,
		Query:       page.QueryParams(),
	}, &internal.Response{
		Result:  &builds,
		Headers: &headers,
	})
	switch err.(type) {
	case nil:
		pagination, err := paginationFromHeaders(headers)
		if err != nil {
			return builds, Pagination{}, false, err
		}

		return builds, pagination, true, nil
	case internal.ResourceNotFoundError:
		return builds, Pagination{}, false, nil
	default:
		return builds, Pagination{}, false, err
	}
}
