package atcclient

import "github.com/concourse/atc"

func (handler AtcHandler) ListAuthMethods() ([]atc.AuthMethod, error) {
	var authMethods []atc.AuthMethod
	err := handler.client.Send(Request{
		RequestName: atc.ListAuthMethods,
	}, &Response{
		Result: &authMethods,
	})

	return authMethods, err
}
