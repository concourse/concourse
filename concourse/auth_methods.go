package concourse

import (
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
)

func (client *client) ListAuthMethods() ([]atc.AuthMethod, error) {
	var authMethods []atc.AuthMethod
	err := client.connection.Send(internal.Request{
		RequestName: atc.ListAuthMethods,
	}, &internal.Response{
		Result: &authMethods,
	})

	return authMethods, err
}
