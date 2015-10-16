package atcclient

import (
	"errors"
	"net/http"

	"github.com/concourse/atc"
)

func (handler AtcHandler) CreateBuild(plan atc.Plan) (atc.Build, error) {
	var build atc.Build
	err := handler.client.MakeRequest(&build, atc.CreateBuild, nil, nil, plan)

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
	err := handler.client.MakeRequest(&build, atc.GetJobBuild, params, nil, nil)

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
	err := handler.client.MakeRequest(&build, atc.GetBuild, params, nil, nil)

	if ure, ok := err.(UnexpectedResponseError); ok {
		if ure.StatusCode == http.StatusNotFound {
			return build, errors.New("build not found")
		}
	}

	return build, err
}

func (handler AtcHandler) AllBuilds() ([]atc.Build, error) {
	var builds []atc.Build
	err := handler.client.MakeRequest(&builds, atc.ListBuilds, nil, nil, nil)
	return builds, err
}

func (handler AtcHandler) AbortBuild(buildID string) error {
	params := map[string]string{"build_id": buildID}
	return handler.client.MakeRequest(nil, atc.AbortBuild, params, nil, nil)
}
