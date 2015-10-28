package concourse

import "github.com/concourse/atc"

func (client *client) AuthToken() (atc.AuthToken, error) {
	var authToken atc.AuthToken
	err := client.connection.Send(Request{
		RequestName: atc.GetAuthToken,
	}, &Response{
		Result: &authToken,
	})

	return authToken, err
}
