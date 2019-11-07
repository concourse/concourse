package concourse

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
)

func (client *client) GetHealth() (atc.Health, error) {
	var atcHealth atc.Health

	err := client.connection.Send(internal.Request{
		RequestName: atc.GetHealth,
	}, &internal.Response{
		Result: &atcHealth,
	})

	return atcHealth, err
}
