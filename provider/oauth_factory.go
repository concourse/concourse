package provider

import (
	"errors"

	"github.com/cloudfoundry/gunk/urljoiner"
	"github.com/concourse/atc/auth/github"
	"github.com/concourse/atc/db"
	"github.com/tedsuo/rata"
)

//go:generate counterfeiter . FactoryDB

type FactoryDB interface {
	GetTeamByName(teamName string) (db.SavedTeam, bool, error)
}

type OAuthFactory struct {
	db             FactoryDB
	atcExternalURL string
	routes         rata.Routes
	callback       string
}

func NewOAuthFactory(db FactoryDB, atcExternalURL string, routes rata.Routes, callback string) OAuthFactory {
	return OAuthFactory{
		db:             db,
		atcExternalURL: atcExternalURL,
		routes:         routes,
		callback:       callback,
	}
}

func (of OAuthFactory) GetProviders(teamName string) (Providers, error) {
	team, found, err := of.db.GetTeamByName(teamName)
	if err != nil {
		return Providers{}, err
	}
	if !found {
		return Providers{}, errors.New("team not found")
	}

	providers := Providers{}

	if len(team.GitHubAuth.Organizations) > 0 ||
		len(team.GitHubAuth.Teams) > 0 ||
		len(team.GitHubAuth.Users) > 0 {

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
