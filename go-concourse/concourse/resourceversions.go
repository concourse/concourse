package concourse

import (
	"net/http"
	"strconv"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) ResourceVersions(pipelineName string, resourceName string, page Page) ([]atc.ResourceVersion, Pagination, bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"resource_name": resourceName,
		"team_name":     team.name,
	}

	var resourceVersions []atc.ResourceVersion
	headers := http.Header{}

	err := team.connection.Send(internal.Request{
		RequestName: atc.ListResourceVersions,
		Params:      params,
		Query:       page.QueryParams(),
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

func (team *team) sendResourceVersion(pipelineName string, resourceName string, resourceVersionID int, resourceVersionReq string) (bool, error) {
	params := rata.Params{
		"pipeline_name":              pipelineName,
		"resource_name":              resourceName,
		"resource_config_version_id": strconv.Itoa(resourceVersionID),
		"team_name":                  team.name,
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
