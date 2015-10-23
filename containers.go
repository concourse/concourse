package atcclient

import "github.com/concourse/atc"

func (handler AtcHandler) ListContainers(queryList map[string]string) ([]atc.Container, error) {
	var containers []atc.Container
	err := handler.client.Send(Request{
		RequestName: atc.ListContainers,
		Queries:     queryList,
	}, &Response{
		Result: &containers,
	})
	return containers, err
}
