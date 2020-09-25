package concourse

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) CheckResource(pipelineName string, resourceName string, version atc.Version) (atc.Check, bool, error) {

	params := rata.Params{
		"pipeline_name": pipelineName,
		"resource_name": resourceName,
		"team_name":     team.Name(),
	}

	var check atc.Check

	jsonBytes, err := json.Marshal(atc.CheckRequestBody{From: version})
	if err != nil {
		return check, false, err
	}

	err = team.connection.Send(internal.Request{
		RequestName: atc.CheckResource,
		Params:      params,
		Body:        bytes.NewBuffer(jsonBytes),
		Header:      http.Header{"Content-Type": []string{"application/json"}},
	}, &internal.Response{
		Result: &check,
	})

	switch e := err.(type) {
	case nil:
		return check, true, nil
	case internal.ResourceNotFoundError:
		return check, false, nil
	case internal.UnexpectedResponseError:
		if e.StatusCode == http.StatusInternalServerError {
			return check, false, GenericError{e.Body}
		} else {
			return check, false, err
		}
	default:
		return check, false, err
	}
}
