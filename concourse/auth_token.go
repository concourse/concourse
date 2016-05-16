package concourse

import (
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (client *client) AuthToken(teamName string) (atc.AuthToken, error) {
	var authToken atc.AuthToken
	err := client.connection.Send(internal.Request{
		RequestName: atc.GetAuthToken,
		Params:      rata.Params{"team_name": teamName},
	}, &internal.Response{
		Result: &authToken,
	})

	return authToken, err
}
