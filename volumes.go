package atcclient

import "github.com/concourse/atc"

func (handler AtcHandler) ListVolumes() ([]atc.Volume, error) {
	var volumes []atc.Volume
	err := handler.client.Send(Request{
		RequestName: atc.ListVolumes,
	}, Response{
		Result: &volumes,
	})
	return volumes, err
}
