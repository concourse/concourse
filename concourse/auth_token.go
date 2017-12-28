package concourse

import (
	"net/url"

	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/concourse/skymarshal/provider"
)

func (team *team) AuthToken() (provider.AuthToken, error) {
	var authToken provider.AuthToken
	err := team.connection.Send(internal.Request{
		RequestName: "GetAuthToken",
		Query:       url.Values{"team_name": []string{team.name}},
	}, &internal.Response{
		Result: &authToken,
	})

	return authToken, err
}
