package concourse

import (
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/concourse/skymarshal/provider"
	"github.com/tedsuo/rata"
)

func (team *team) AuthToken() (provider.AuthToken, error) {
	var authToken provider.AuthToken
	err := team.connection.Send(internal.Request{
		RequestName: atc.LegacyGetAuthToken,
		Params:      rata.Params{"team_name": team.name},
	}, &internal.Response{
		Result: &authToken,
	})

	return authToken, err
}
