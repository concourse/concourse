package atcclient

import "github.com/concourse/atc"

func (client *client) ListVolumes() ([]atc.Volume, error) {
	var volumes []atc.Volume
	err := client.connection.Send(Request{
		RequestName: atc.ListVolumes,
	}, &Response{
		Result: &volumes,
	})
	return volumes, err
}
