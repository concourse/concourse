package atcclient

import "github.com/concourse/atc"

func (handler AtcHandler) CreatePipe() (atc.Pipe, error) {
	var pipe atc.Pipe
	err := handler.client.MakeRequest(&pipe, atc.CreatePipe, nil, nil, nil)

	return pipe, err
}
