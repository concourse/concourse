package concourse

import (
	"net/url"

	"github.com/concourse/atc"
)

func (client *client) ListContainers(queryList map[string]string) ([]atc.Container, error) {
	var containers []atc.Container
	urlValues := url.Values{}

	for k, v := range queryList {
		urlValues[k] = []string{v}
	}
	err := client.connection.Send(Request{
		RequestName: atc.ListContainers,
		Query:       urlValues,
	}, &Response{
		Result: &containers,
	})
	return containers, err
}
