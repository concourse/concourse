package provider

import (
	"errors"

	"github.com/cloudfoundry/gunk/urljoiner"
	"github.com/concourse/atc/auth/github"
	"github.com/concourse/atc/db"
	"github.com/tedsuo/rata"
)

type OAuthFactory struct {
	teamDBFactory  db.TeamDBFactory
	atcExternalURL string
	routes         rata.Routes
	callback       string
}

func NewOAuthFactory(teamDBFactory db.TeamDBFactory, atcExternalURL string, routes rata.Routes, callback string) OAuthFactory {
	return OAuthFactory{
		teamDBFactory:  teamDBFactory,
		atcExternalURL: atcExternalURL,
		routes:         routes,
		callback:       callback,
	}
}

func (of OAuthFactory) GetProviders(teamName string) (Providers, error) {
	teamDB := of.teamDBFactory.GetTeamDB(teamName)
	team, found, err := teamDB.GetTeam()
	if err != nil {
		return Providers{}, err
	}
	if !found {
		return Providers{}, errors.New("team not found")
	}

	providers := Providers{}

	if team.GitHubAuth != nil {
		redirectURL, err := of.routes.CreatePathForRoute(of.callback, rata.Params{
			"provider": github.ProviderName,
		})
		if err != nil {
			return Providers{}, err
		}
		gitHubAuthProvider := github.NewProvider(team.GitHubAuth, urljoiner.Join(of.atcExternalURL, redirectURL))

		providers[github.ProviderName] = gitHubAuthProvider
	}

	return providers, err
}
