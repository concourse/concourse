package concourse

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

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

	switch e := err.(type) {
	case nil:
		return true, nil
	case internal.ResourceNotFoundError:
		return false, nil
	case internal.UnexpectedResponseError:
		switch e.StatusCode {
		case http.StatusBadRequest:
			var checkRes atc.CheckResponseBody
			err = json.Unmarshal([]byte(e.Body), &checkRes)
			if err != nil {
				return false, err
			}

			return false, CommandFailedError{
				Command:    "check",
				ExitStatus: checkRes.ExitStatus,
				Output:     checkRes.Stderr,
			}
		case http.StatusInternalServerError:
			return false, GenericError{
				e.Body,
			}
		default:
			return false, err
		}
	default:
		return false, err
	}
}
