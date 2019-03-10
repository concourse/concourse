package concourse

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) CheckResourceType(pipelineName string, resourceTypeName string, version atc.Version) (bool, error) {
	params := rata.Params{
		"pipeline_name":      pipelineName,
		"resource_type_name": resourceTypeName,
		"team_name":          team.name,
	}

	jsonBytes, err := json.Marshal(atc.CheckRequestBody{From: version})
	if err != nil {
		return false, err
	}

	response := internal.Response{}
	err = team.connection.Send(internal.Request{
		ReturnResponseBody: true,
		RequestName:        atc.CheckResourceType,
		Params:             params,
		Body:               bytes.NewBuffer(jsonBytes),
		Header:             http.Header{"Content-Type": []string{"application/json"}},
	}, &response)

	switch e := err.(type) {
	case nil:
		return true, nil
	case internal.ResourceNotFoundError:
		return false, nil
	case internal.UnexpectedResponseError:
		if e.StatusCode == http.StatusInternalServerError {
			return false, GenericError{
				e.Body,
			}
		} else {
			return false, err
		}
	default:
		return false, err
	}
}
