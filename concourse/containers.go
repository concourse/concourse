package concourse

import "github.com/concourse/atc"

func (client *client) ListContainers(queryList map[string]string) ([]atc.Container, error) {
	var containers []atc.Container
	err := client.connection.Send(Request{
		RequestName: atc.ListContainers,
		Queries:     queryList,
	}, &Response{
		Result: &containers,
	})
	return containers, err
}
