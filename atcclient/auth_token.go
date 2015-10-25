package atcclient

import "github.com/concourse/atc"

func (handler AtcHandler) AuthToken() (atc.AuthToken, error) {
	var authToken atc.AuthToken
	err := handler.client.Send(Request{
		RequestName: atc.GetAuthToken,
	}, &Response{
		Result: &authToken,
	})

	return authToken, err
}
