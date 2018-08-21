package concourse

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) CheckResourceType(pipelineName string, resourceTypeName string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"resource_name": resourceTypeName,
		"team_name":     team.name,
	}

	response := internal.Response{}
	err := team.connection.Send(internal.Request{
		ReturnResponseBody: true,
		RequestName:        atc.CheckResourceType,
		Params:             params,
		Header:             http.Header{"Content-Type": []string{"application/json"}},
	}, &response)

	switch err.(type) {
	case nil:
		return true, nil
	case internal.ResourceNotFoundError:
		return false, nil
	default:
		if unexpectedResponseError, ok := err.(internal.UnexpectedResponseError); ok {
			if unexpectedResponseError.StatusCode == http.StatusInternalServerError {
				checkResourceErr := CheckResourceError{
					atc.CheckResponseBody{
						Stderr:     unexpectedResponseError.Body,
						ExitStatus: 70,
					},
				}

				return false, checkResourceErr
			}
		}

		return false, err
	}
}
