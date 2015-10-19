package atcclient

import "github.com/concourse/atc"

func (handler AtcHandler) ListWorkers() ([]atc.Worker, error) {
	var workers []atc.Worker
	err := handler.client.Send(Request{
		RequestName: atc.ListWorkers,
	}, Response{
		Result: &workers,
	})
	return workers, err
}
