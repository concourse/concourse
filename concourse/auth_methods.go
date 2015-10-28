package concourse

import "github.com/concourse/atc"

func (client *client) ListAuthMethods() ([]atc.AuthMethod, error) {
	var authMethods []atc.AuthMethod
	err := client.connection.Send(Request{
		RequestName: atc.ListAuthMethods,
	}, &Response{
		Result: &authMethods,
	})

	return authMethods, err
}
