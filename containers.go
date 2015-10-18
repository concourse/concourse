package atcclient

import "github.com/concourse/atc"

func (handler AtcHandler) ListContainers() ([]atc.Container, error) {
	var containers []atc.Container
	err := handler.client.Send(Request{
		RequestName: atc.ListContainers,
		Result:      &containers,
	})
	return containers, err
}
