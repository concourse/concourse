package concourse

import (
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) AuthToken() (atc.AuthToken, error) {
	var authToken atc.AuthToken
	err := team.connection.Send(internal.Request{
		RequestName: atc.GetAuthToken,
		Params:      rata.Params{"team_name": team.name},
	}, &internal.Response{
		Result: &authToken,
	})

	return authToken, err
}
