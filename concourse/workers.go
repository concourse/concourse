package concourse

import (
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
)

func (client *client) ListWorkers() ([]atc.Worker, error) {
	var workers []atc.Worker
	err := client.connection.Send(internal.Request{
		RequestName: atc.ListWorkers,
	}, &internal.Response{
		Result: &workers,
	})
	return workers, err
}
