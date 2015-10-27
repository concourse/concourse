package atcclient

import "github.com/concourse/atc"

func (client *client) CreatePipe() (atc.Pipe, error) {
	var pipe atc.Pipe
	err := client.connection.Send(Request{
		RequestName: atc.CreatePipe,
	}, &Response{
		Result: &pipe,
	})

	return pipe, err
}
