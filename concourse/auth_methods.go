package concourse

import (
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (client *client) ListAuthMethods(teamName string) ([]atc.AuthMethod, error) {
	var authMethods []atc.AuthMethod
	err := client.connection.Send(internal.Request{
		RequestName: atc.ListAuthMethods,
		Params:      rata.Params{"team_name": teamName},
	}, &internal.Response{
		Result: &authMethods,
	})

	return authMethods, err
}
