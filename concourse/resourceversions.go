package concourse

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (client *client) ResourceVersions(pipelineName string, resourceName string, page Page) ([]atc.VersionedResource, Pagination, bool, error) {
	params := rata.Params{"pipeline_name": pipelineName, "resource_name": resourceName}
	var versionedResources []atc.VersionedResource
	headers := http.Header{}

	err := client.connection.Send(internal.Request{
		RequestName: atc.ListResourceVersions,
		Params:      params,
		Query:       page.QueryParams(),
	}, &internal.Response{
		Result:  &versionedResources,
		Headers: &headers,
	})
	switch err.(type) {
	case nil:
		pagination, err := paginationFromHeaders(headers)
		if err != nil {
			return versionedResources, Pagination{}, false, err
		}

		return versionedResources, pagination, true, nil
	case internal.ResourceNotFoundError:
		return versionedResources, Pagination{}, false, nil
	default:
		return versionedResources, Pagination{}, false, err
	}
}
