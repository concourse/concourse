package concourse

import (
	"net/url"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
)

func (client *client) ListContainers(queryList map[string]string) ([]atc.Container, error) {
	var containers []atc.Container
	urlValues := url.Values{}

	for k, v := range queryList {
		urlValues[k] = []string{v}
	}
	err := client.connection.Send(internal.Request{
		RequestName: atc.ListContainers,
		Query:       urlValues,
	}, &internal.Response{
		Result: &containers,
	})
	return containers, err
}
