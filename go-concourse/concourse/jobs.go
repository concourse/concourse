package concourse

import (
	"net/http"
	"net/url"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) ListJobs(pipelineRef atc.PipelineRef) ([]atc.Job, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
		"team_name":     team.Name(),
	}

	var jobs []atc.Job
	err := team.connection.Send(internal.Request{
		RequestName: atc.ListJobs,
		Params:      params,
		Query:       pipelineRef.QueryParams(),
	}, &internal.Response{
		Result: &jobs,
	})

	return jobs, err
}

func (client *client) ListAllJobs() ([]atc.Job, error) {
	var jobs []atc.Job
	err := client.connection.Send(internal.Request{
		RequestName: atc.ListAllJobs,
	}, &internal.Response{
		Result: &jobs,
	})

	return jobs, err
}

func (team *team) Job(pipelineRef atc.PipelineRef, jobName string) (atc.Job, bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
		"job_name":      jobName,
		"team_name":     team.Name(),
	}

	var job atc.Job
	err := team.connection.Send(internal.Request{
		RequestName: atc.GetJob,
		Params:      params,
		Query:       pipelineRef.QueryParams(),
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

func (team *team) JobBuilds(pipelineRef atc.PipelineRef, jobName string, page Page) ([]atc.Build, Pagination, bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
		"job_name":      jobName,
		"team_name":     team.Name(),
	}

	var builds []atc.Build

	headers := http.Header{}
	err := team.connection.Send(internal.Request{
		RequestName: atc.ListJobBuilds,
		Params:      params,
		Query:       merge(page.QueryParams(), pipelineRef.QueryParams()),
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

func (team *team) PauseJob(pipelineRef atc.PipelineRef, jobName string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
		"job_name":      jobName,
		"team_name":     team.Name(),
	}

	err := team.connection.Send(internal.Request{
		RequestName: atc.PauseJob,
		Params:      params,
		Query:       pipelineRef.QueryParams(),
	}, &internal.Response{})

	switch err.(type) {
	case nil:
		return true, nil
	case internal.ResourceNotFoundError:
		return false, nil
	default:
		return false, err
	}
}

func (team *team) UnpauseJob(pipelineRef atc.PipelineRef, jobName string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
		"job_name":      jobName,
		"team_name":     team.Name(),
	}

	err := team.connection.Send(internal.Request{
		RequestName: atc.UnpauseJob,
		Params:      params,
		Query:       pipelineRef.QueryParams(),
	}, &internal.Response{})

	switch err.(type) {
	case nil:
		return true, nil
	case internal.ResourceNotFoundError:
		return false, nil
	default:
		return false, err
	}
}

func (team *team) ScheduleJob(pipelineRef atc.PipelineRef, jobName string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
		"job_name":      jobName,
		"team_name":     team.Name(),
	}

	err := team.connection.Send(internal.Request{
		RequestName: atc.ScheduleJob,
		Params:      params,
		Query:       pipelineRef.QueryParams(),
	}, &internal.Response{})

	switch err.(type) {
	case nil:
		return true, nil
	case internal.ResourceNotFoundError:
		return false, nil
	default:
		return false, err
	}
}

func (team *team) ClearTaskCache(pipelineRef atc.PipelineRef, jobName string, stepName string, cachePath string) (int64, error) {
	params := rata.Params{
		"team_name":     team.Name(),
		"pipeline_name": pipelineRef.Name,
		"job_name":      jobName,
		"step_name":     stepName,
	}

	queryParams := url.Values{}
	if len(cachePath) > 0 {
		queryParams.Add(atc.ClearTaskCacheQueryPath, cachePath)
	}

	var ctcResponse atc.ClearTaskCacheResponse
	responseHeaders := http.Header{}
	response := internal.Response{
		Headers: &responseHeaders,
		Result:  &ctcResponse,
	}
	err := team.connection.Send(internal.Request{
		RequestName: atc.ClearTaskCache,
		Params:      params,
		Query:       merge(queryParams, pipelineRef.QueryParams()),
	}, &response)

	if err != nil {
		return 0, err
	} else {
		return ctcResponse.CachesRemoved, nil
	}
}
