package concourse

import (
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
)

func (client *client) CreatePipe() (atc.Pipe, error) {
	var pipe atc.Pipe
	err := client.connection.Send(internal.Request{
		RequestName: atc.CreatePipe,
	}, &internal.Response{
		Result: &pipe,
	})

	return pipe, err
}
