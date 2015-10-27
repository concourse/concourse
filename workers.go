package atcclient

import "github.com/concourse/atc"

func (client *client) ListWorkers() ([]atc.Worker, error) {
	var workers []atc.Worker
	err := client.connection.Send(Request{
		RequestName: atc.ListWorkers,
	}, &Response{
		Result: &workers,
	})
	return workers, err
}
