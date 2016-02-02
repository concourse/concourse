package concourse

import (
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
)

func (client *client) AuthToken() (atc.AuthToken, error) {
	var authToken atc.AuthToken
	err := client.connection.Send(internal.Request{
		RequestName: atc.GetAuthToken,
	}, &internal.Response{
		Result: &authToken,
	})

	return authToken, err
}
