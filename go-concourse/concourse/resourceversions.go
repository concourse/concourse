package concourse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) ResourceVersions(pipelineRef atc.PipelineRef, resourceName string, page Page, filter atc.Version) ([]atc.ResourceVersion, Pagination, bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
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
		Query:       merge(queryParams, pipelineRef.QueryParams()),
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

func (team *team) DisableResourceVersion(pipelineRef atc.PipelineRef, resourceName string, resourceVersionID int) (bool, error) {
	return team.sendResourceVersion(pipelineRef, resourceName, resourceVersionID, atc.DisableResourceVersion)
}

func (team *team) EnableResourceVersion(pipelineRef atc.PipelineRef, resourceName string, resourceVersionID int) (bool, error) {
	return team.sendResourceVersion(pipelineRef, resourceName, resourceVersionID, atc.EnableResourceVersion)
}

func (team *team) PinResourceVersion(pipelineRef atc.PipelineRef, resourceName string, resourceVersionID int) (bool, error) {
	return team.sendResourceVersion(pipelineRef, resourceName, resourceVersionID, atc.PinResourceVersion)
}

func (team *team) UnpinResource(pipelineRef atc.PipelineRef, resourceName string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
		"resource_name": resourceName,
		"team_name":     team.Name(),
	}

	err := team.connection.Send(internal.Request{
		RequestName: atc.UnpinResource,
		Params:      params,
		Query:       pipelineRef.QueryParams(),
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

func (team *team) SetPinComment(pipelineRef atc.PipelineRef, resourceName string, comment string) (bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
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
		Query:  pipelineRef.QueryParams(),
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

func (team *team) sendResourceVersion(pipelineRef atc.PipelineRef, resourceName string, resourceVersionID int, resourceVersionReq string) (bool, error) {
	params := rata.Params{
		"pipeline_name":              pipelineRef.Name,
		"resource_name":              resourceName,
		"resource_config_version_id": strconv.Itoa(resourceVersionID),
		"team_name":                  team.Name(),
	}

	err := team.connection.Send(internal.Request{
		RequestName: resourceVersionReq,
		Params:      params,
		Query:       pipelineRef.QueryParams(),
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

func (team *team) ClearResourceVersions(pipelineRef atc.PipelineRef, ResourceName string) (int64, error) {
	params := rata.Params{
		"team_name":     team.Name(),
		"pipeline_name": pipelineRef.Name,
		"resource_name": ResourceName,
	}

	queryParams := url.Values{}
	var crvResponse atc.ClearVersionsResponse
	responseHeaders := http.Header{}
	response := internal.Response{
		Headers: &responseHeaders,
		Result:  &crvResponse,
	}
	request := internal.Request{
		RequestName: atc.ClearResourceVersions,
		Params:      params,
		Query:       merge(queryParams, pipelineRef.QueryParams()),
		Header:      http.Header{"Content-Type": []string{"application/json"}},
	}
	err := team.connection.Send(request, &response)
	if err != nil {
		return 0, err
	} else {
		return crvResponse.VersionsRemoved, nil
	}
}

func (team *team) ClearResourceTypeVersions(pipelineRef atc.PipelineRef, ResourceTypeName string) (int64, error) {
	params := rata.Params{
		"team_name":          team.Name(),
		"pipeline_name":      pipelineRef.Name,
		"resource_type_name": ResourceTypeName,
	}

	queryParams := url.Values{}
	var crvResponse atc.ClearVersionsResponse
	responseHeaders := http.Header{}
	response := internal.Response{
		Headers: &responseHeaders,
		Result:  &crvResponse,
	}
	request := internal.Request{
		RequestName: atc.ClearResourceTypeVersions,
		Params:      params,
		Query:       merge(queryParams, pipelineRef.QueryParams()),
		Header:      http.Header{"Content-Type": []string{"application/json"}},
	}
	err := team.connection.Send(request, &response)
	if err != nil {
		return 0, err
	} else {
		return crvResponse.VersionsRemoved, nil
	}
}
