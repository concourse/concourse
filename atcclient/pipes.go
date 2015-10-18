package atcclient

import "github.com/concourse/atc"

func (handler AtcHandler) CreatePipe() (atc.Pipe, error) {
	var pipe atc.Pipe
	err := handler.client.Send(Request{
		RequestName: atc.CreatePipe,
		Result:      &pipe,
	})

	return pipe, err
}
