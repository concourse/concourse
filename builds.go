package atcclient

import (
	"errors"
	"net/http"

	"github.com/concourse/atc"
)

func (handler AtcHandler) CreateBuild(plan atc.Plan) (atc.Build, error) {
	var build atc.Build
	err := handler.client.Send(Request{
		Result:      &build,
		RequestName: atc.CreateBuild,
		Body:        plan,
	})

	if ure, ok := err.(UnexpectedResponseError); ok {
		if ure.StatusCode == http.StatusNotFound {
			return build, errors.New("build not found")
		}
	}

	return build, err
}

func (handler AtcHandler) JobBuild(pipelineName, jobName, buildName string) (atc.Build, error) {
	if pipelineName == "" {
		pipelineName = atc.DefaultPipelineName
	}
	params := map[string]string{"job_name": jobName, "build_name": buildName, "pipeline_name": pipelineName}
	var build atc.Build
	err := handler.client.Send(Request{
		RequestName: atc.GetJobBuild,
		Params:      params,
		Result:      &build,
	})

	if ure, ok := err.(UnexpectedResponseError); ok {
		if ure.StatusCode == http.StatusNotFound {
			return build, errors.New("build not found")
		}
	}

	return build, err
}

func (handler AtcHandler) Build(buildID string) (atc.Build, error) {
	params := map[string]string{"build_id": buildID}
	var build atc.Build
	err := handler.client.Send(Request{
		RequestName: atc.GetBuild,
		Params:      params,
		Result:      &build,
	})

	if ure, ok := err.(UnexpectedResponseError); ok {
		if ure.StatusCode == http.StatusNotFound {
			return build, errors.New("build not found")
		}
	}

	return build, err
}

func (handler AtcHandler) AllBuilds() ([]atc.Build, error) {
	var builds []atc.Build
	err := handler.client.Send(Request{
		RequestName: atc.ListBuilds,
		Result:      &builds,
	})
	return builds, err
}

func (handler AtcHandler) AbortBuild(buildID string) error {
	params := map[string]string{"build_id": buildID}
	return handler.client.Send(Request{
		RequestName: atc.AbortBuild,
		Params:      params,
	})

}
