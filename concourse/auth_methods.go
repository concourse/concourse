package concourse

import (
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/concourse/skymarshal/provider"
	"github.com/tedsuo/rata"
)

func (team *team) ListAuthMethods() ([]provider.AuthMethod, error) {
	var authMethods []provider.AuthMethod
	err := team.connection.Send(internal.Request{
		RequestName: atc.LegacyListAuthMethods,
		Params:      rata.Params{"team_name": team.name},
	}, &internal.Response{
		Result: &authMethods,
	})

	return authMethods, err
}
