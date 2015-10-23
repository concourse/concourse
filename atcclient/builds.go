package atcclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/concourse/atc"
)

func (handler AtcHandler) CreateBuild(plan atc.Plan) (atc.Build, error) {
	var build atc.Build

	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(plan)
	if err != nil {
		return build, fmt.Errorf("Unable to marshal plan: %s", err)
	}

	err = handler.client.Send(Request{
		RequestName: atc.CreateBuild,
		Body:        buffer,
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
	}, &Response{
		Result: &build,
	})

	if ure, ok := err.(UnexpectedResponseError); ok {
		if ure.StatusCode == http.StatusNotFound {
			return build, errors.New("build not found")
		}
	}

	return build, err
}

func (handler AtcHandler) JobBuild(pipelineName, jobName, buildName string) (atc.Build, bool, error) {
	if pipelineName == "" {
		return atc.Build{}, false, NameRequiredError("pipeline")
	}

	params := map[string]string{"job_name": jobName, "build_name": buildName, "pipeline_name": pipelineName}
	var build atc.Build
	err := handler.client.Send(Request{
		RequestName: atc.GetJobBuild,
		Params:      params,
	}, &Response{
		Result: &build,
	})

	switch err.(type) {
	case nil:
		return build, true, nil
	case ResourceNotFoundError:
		return build, false, nil
	default:
		return build, false, err
	}
}

func (handler AtcHandler) Build(buildID string) (atc.Build, bool, error) {
	params := map[string]string{"build_id": buildID}
	var build atc.Build
	err := handler.client.Send(Request{
		RequestName: atc.GetBuild,
		Params:      params,
	}, &Response{
		Result: &build,
	})

	switch err.(type) {
	case nil:
		return build, true, nil
	case ResourceNotFoundError:
		return build, false, nil
	default:
		return build, false, err
	}
}

func (handler AtcHandler) AllBuilds() ([]atc.Build, error) {
	var builds []atc.Build
	err := handler.client.Send(Request{
		RequestName: atc.ListBuilds,
	}, &Response{
		Result: &builds,
	})
	return builds, err
}

func (handler AtcHandler) AbortBuild(buildID string) error {
	params := map[string]string{"build_id": buildID}
	return handler.client.Send(Request{
		RequestName: atc.AbortBuild,
		Params:      params,
	}, nil)

}
