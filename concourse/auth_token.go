package concourse

import (
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (client *client) AuthToken() (atc.AuthToken, error) {
	var authToken atc.AuthToken
	err := client.connection.Send(internal.Request{
		RequestName: atc.GetAuthToken,
		Params:      rata.Params{"team_name": atc.DefaultTeamName},
	}, &internal.Response{
		Result: &authToken,
	})

	return authToken, err
}
