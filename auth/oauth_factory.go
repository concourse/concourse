package auth

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/urljoiner"
	"github.com/concourse/atc/db"
	"github.com/concourse/skymarshal/provider"
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

func (of OAuthFactory) GetProvider(team db.Team, providerName string) (provider.Provider, bool, error) {
	redirectURL, err := of.routes.CreatePathForRoute(of.callback, rata.Params{
		"provider": providerName,
	})
	if err != nil {
		of.logger.Error("failed-to-construct-redirect-url", err, lager.Data{"provider": providerName})
		return nil, false, err
	}

	auth, found := team.Auth()[providerName]
	if !found {
		return nil, false, nil
	}

	oauthProvider, found := provider.NewProvider(auth, providerName, urljoiner.Join(of.atcExternalURL, redirectURL))
	if !found {
		return nil, false, nil
	}

	return oauthProvider, true, nil
}
