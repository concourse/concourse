package provider

import (
	"code.cloudfoundry.org/urljoiner"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/auth/genericoauth"
	"github.com/concourse/atc/auth/github"
	"github.com/concourse/atc/auth/uaa"
	"github.com/concourse/atc/db"
	"github.com/tedsuo/rata"
)

type OAuthFactory struct {
	logger         lager.Logger
	atcExternalURL string
	routes         rata.Routes
	callback       string
}

func NewOAuthFactory(logger lager.Logger, atcExternalURL string, routes rata.Routes, callback string) OAuthFactory {
	return OAuthFactory{
		logger:         logger,
		atcExternalURL: atcExternalURL,
		routes:         routes,
		callback:       callback,
	}
}

func (of OAuthFactory) GetProvider(team db.SavedTeam, providerName string) (Provider, bool, error) {
	redirectURL, err := of.routes.CreatePathForRoute(of.callback, rata.Params{
		"provider": providerName,
	})
	if err != nil {
		of.logger.Error("failed-to-construct-redirect-url", err, lager.Data{"provider": providerName})
		return nil, false, err
	}

	switch providerName {
	case github.ProviderName:
		if team.GitHubAuth == nil {
			return nil, false, nil
		}

		return github.NewProvider(team.GitHubAuth, urljoiner.Join(of.atcExternalURL, redirectURL)), true, nil

	case uaa.ProviderName:
		if team.UAAAuth == nil {
			of.logger.Error("failed-to-construct-redirect-url", err, lager.Data{"provider": providerName})
			return nil, false, nil
		}

		return uaa.NewProvider(team.UAAAuth, urljoiner.Join(of.atcExternalURL, redirectURL)), true, nil

	case genericoauth.ProviderName:
		if team.GenericOAuth == nil {
			return nil, false, nil
		}

		return genericoauth.NewProvider(team.GenericOAuth, urljoiner.Join(of.atcExternalURL, redirectURL)), true, nil

	}

	return nil, false, nil
}
