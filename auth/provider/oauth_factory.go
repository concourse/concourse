package provider

import (
	"github.com/cloudfoundry/gunk/urljoiner"
	"github.com/concourse/atc/auth/github"
	"github.com/concourse/atc/db"
	"github.com/tedsuo/rata"
)

//go:generate counterfeiter . FactoryDB

type FactoryDB interface {
	GetTeamByName(teamName string) (db.SavedTeam, error)
}

type OauthFactory struct {
	db             FactoryDB
	atcExternalURL string
	routes         rata.Routes
	callback       string
}

func NewOauthFactory(db FactoryDB, atcExternalURL string, routes rata.Routes, callback string) OauthFactory {
	return OauthFactory{
		db:             db,
		atcExternalURL: atcExternalURL,
		routes:         routes,
		callback:       callback,
	}
}

func (of OauthFactory) GetProviders(teamName string) (Providers, error) {
	team, err := of.db.GetTeamByName(teamName)
	if err != nil {
		return Providers{}, err
	}

	providers := Providers{}
	redirectURL, err := of.routes.CreatePathForRoute(of.callback, rata.Params{
		"provider": github.ProviderName,
	})
	if err != nil {
		return Providers{}, err
	}
	githubAuthProvider := github.NewProvider(team.GitHubAuth, urljoiner.Join(of.atcExternalURL, redirectURL))

	providers[github.ProviderName] = githubAuthProvider
	return providers, err
}
