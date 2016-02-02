package concourse

import (
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
)

func (client *client) ListVolumes() ([]atc.Volume, error) {
	var volumes []atc.Volume
	err := client.connection.Send(internal.Request{
		RequestName: atc.ListVolumes,
	}, &internal.Response{
		Result: &volumes,
	})
	return volumes, err
}
