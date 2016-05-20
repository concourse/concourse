package concourse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

type CheckResourceError struct {
	atc.CheckResponseBody
}

func (checkResourceError CheckResourceError) Error() string {
	return fmt.Sprintf("check failed with exit status '%d':\n%s\n", checkResourceError.ExitStatus, checkResourceError.Stderr)
}

func (team *team) CheckResource(pipelineName string, resourceName string, version atc.Version) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"resource_name": resourceName,
		"team_name":     team.name,
	}

	jsonBytes, err := json.Marshal(atc.CheckRequestBody{From: version})
	if err != nil {
		return false, err
	}

	response := internal.Response{}
	err = team.connection.Send(internal.Request{
		ReturnResponseBody: true,
		RequestName:        atc.CheckResource,
		Params:             params,
		Body:               bytes.NewBuffer(jsonBytes),
		Header:             http.Header{"Content-Type": []string{"application/json"}},
	}, &response)

	switch err.(type) {
	case nil:
		return true, nil
	case internal.ResourceNotFoundError:
		return false, nil
	default:
		if unexpectedResponseError, ok := err.(internal.UnexpectedResponseError); ok {
			if unexpectedResponseError.StatusCode == http.StatusBadRequest {
				var checkResourceErr CheckResourceError

				err = json.Unmarshal([]byte(unexpectedResponseError.Body), &checkResourceErr)
				if err != nil {
					return false, err
				}

				return false, checkResourceErr
			}
		}

		return false, err
	}
}
