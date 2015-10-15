package atcclient

import (
	"errors"
	"net/http"

	"github.com/concourse/atc"
)

func (buildHandler AtcHandler) JobBuild(pipelineName, jobName, buildName string) (atc.Build, error) {
	if pipelineName == "" {
		pipelineName = atc.DefaultPipelineName
	}
	params := map[string]string{"job_name": jobName, "build_name": buildName, "pipeline_name": pipelineName}
	var build atc.Build
	err := buildHandler.client.MakeRequest(&build, atc.GetJobBuild, params, nil)

	if ure, ok := err.(UnexpectedResponseError); ok {
		if ure.StatusCode == http.StatusNotFound {
			return build, errors.New("build not found")
		}
	}

	return build, err
}

func (buildHandler AtcHandler) Build(buildID string) (atc.Build, error) {
	params := map[string]string{"build_id": buildID}
	var build atc.Build
	err := buildHandler.client.MakeRequest(&build, atc.GetBuild, params, nil)

	if ure, ok := err.(UnexpectedResponseError); ok {
		if ure.StatusCode == http.StatusNotFound {
			return build, errors.New("build not found")
		}
	}

	return build, err
}

func (buildHandler AtcHandler) AllBuilds() ([]atc.Build, error) {
	var builds []atc.Build
	err := buildHandler.client.MakeRequest(&builds, atc.ListBuilds, nil, nil)
	return builds, err
}
