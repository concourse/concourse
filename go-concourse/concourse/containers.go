package concourse

import (
	"net/url"

	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) ListContainers(queryList map[string]string) ([]atc.Container, error) {
	var containers []atc.Container
	urlValues := url.Values{}

	params := rata.Params{
		"team_name": team.name,
	}
	for k, v := range queryList {
		urlValues[k] = []string{v}
	}
	err := team.connection.Send(internal.Request{
		RequestName: atc.ListContainers,
		Query:       urlValues,
		Params:      params,
	}, &internal.Response{
		Result: &containers,
	})
	return containers, err
}

func (team *team) GetContainer(handle string) (atc.Container, error) {
	var container atc.Container

	params := rata.Params{
		"id":        handle,
		"team_name": team.name,
	}

	err := team.connection.Send(internal.Request{
		RequestName: atc.GetContainer,
		Params:      params,
	}, &internal.Response{
		Result: &container,
	})

	return container, err
}
