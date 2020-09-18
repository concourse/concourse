package concourse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) ResourceVersions(pipelineName string, resourceName string, page Page, filter atc.Version) ([]atc.ResourceVersion, Pagination, bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"resource_name": resourceName,
		"team_name":     team.Name(),
	}

	var resourceVersions []atc.ResourceVersion
	headers := http.Header{}

	queryParams := page.QueryParams()
	for k, v := range filter {
		queryParams.Add("filter", fmt.Sprintf("%s:%s", k, v))
	}

	err := team.connection.Send(internal.Request{
		RequestName: atc.ListResourceVersions,
		Params:      params,
		Query:       queryParams,
	}, &internal.Response{
		Result:  &resourceVersions,
		Headers: &headers,
	})
	switch err.(type) {
	case nil:
		pagination, err := paginationFromHeaders(headers)
		if err != nil {
			return resourceVersions, Pagination{}, false, err
		}

		return resourceVersions, pagination, true, nil
	case internal.ResourceNotFoundError:
		return resourceVersions, Pagination{}, false, nil
	default:
		return resourceVersions, Pagination{}, false, err
	}
}

func (team *team) DisableResourceVersion(pipelineName string, resourceName string, resourceVersionID int) (bool, error) {
	return team.sendResourceVersion(pipelineName, resourceName, resourceVersionID, atc.DisableResourceVersion)
}

func (team *team) EnableResourceVersion(pipelineName string, resourceName string, resourceVersionID int) (bool, error) {
	return team.sendResourceVersion(pipelineName, resourceName, resourceVersionID, atc.EnableResourceVersion)
}

func (team *team) PinResourceVersion(pipelineName string, resourceName string, resourceVersionID int) (bool, error) {
	return team.sendResourceVersion(pipelineName, resourceName, resourceVersionID, atc.PinResourceVersion)
}

func (team *team) UnpinResource(pipelineName string, resourceName string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"resource_name": resourceName,
		"team_name":     team.Name(),
	}

	err := team.connection.Send(internal.Request{
		RequestName: atc.UnpinResource,
		Params:      params,
	}, nil)

	switch err.(type) {
	case nil:
		return true, nil
	case internal.ResourceNotFoundError:
		return false, nil
	default:
		return false, err
	}
}

func (team *team) SetPinComment(pipelineName string, resourceName string, comment string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"resource_name": resourceName,
		"team_name":     team.Name(),
	}

	pinComment := atc.SetPinCommentRequestBody{
		PinComment: comment,
	}

	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(pinComment)
	if err != nil {
		return false, fmt.Errorf("Unable to marshal comment: %s", err)
	}

	err = team.connection.Send(internal.Request{
		RequestName: atc.SetPinCommentOnResource,
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
		Params: params,
		Body:   buffer,
	}, nil)

	switch err.(type) {
	case nil:
		return true, nil
	case internal.ResourceNotFoundError:
		return false, nil
	default:
		return false, err
	}

}

func (team *team) sendResourceVersion(pipelineName string, resourceName string, resourceVersionID int, resourceVersionReq string) (bool, error) {
	params := rata.Params{
		"pipeline_name":              pipelineName,
		"resource_name":              resourceName,
		"resource_config_version_id": strconv.Itoa(resourceVersionID),
		"team_name":                  team.Name(),
	}

	err := team.connection.Send(internal.Request{
		RequestName: resourceVersionReq,
		Params:      params,
	}, nil)

	switch err.(type) {
	case nil:
		return true, nil
	case internal.ResourceNotFoundError:
		return false, nil
	default:
		return false, err
	}
}
