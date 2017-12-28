package concourse

import (
	"net/url"

	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/concourse/skymarshal/provider"
)

func (team *team) ListAuthMethods() ([]provider.AuthMethod, error) {
	var authMethods []provider.AuthMethod
	err := team.connection.Send(internal.Request{
		RequestName: "ListAuthMethods",
		Query:       url.Values{"team_name": []string{team.name}},
	}, &internal.Response{
		Result: &authMethods,
	})

	return authMethods, err
}
