package concourse

import (
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) ListAuthMethods() ([]atc.AuthMethod, error) {
	var authMethods []atc.AuthMethod
	err := team.connection.Send(internal.Request{
		RequestName: atc.ListAuthMethods,
		Params:      rata.Params{"team_name": team.name},
	}, &internal.Response{
		Result: &authMethods,
	})

	return authMethods, err
}
