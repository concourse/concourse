package auth

import (
	"crypto/rsa"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/db"
	"github.com/dgrijalva/jwt-go"
	"github.com/tedsuo/rata"
)

var SigningMethod = jwt.SigningMethodRS256

//go:generate counterfeiter . ProviderFactory

type ProviderFactory interface {
	GetProvider(db.SavedTeam, string) (provider.Provider, bool, error)
}

func NewOAuthHandler(
	logger lager.Logger,
	providerFactory ProviderFactory,
	teamDBFactory db.TeamDBFactory,
	signingKey *rsa.PrivateKey,
) (http.Handler, error) {
	return rata.NewRouter(
		OAuthRoutes,
		map[string]http.Handler{
			OAuthBegin: NewOAuthBeginHandler(
				logger.Session("oauth-begin"),
				providerFactory,
				signingKey,
				teamDBFactory,
			),
			OAuthCallback: NewOAuthCallbackHandler(
				logger.Session("oauth-callback"),
				providerFactory,
				signingKey,
				teamDBFactory,
			),
			LogOut: NewLogOutHandler(
				logger.Session("logout"),
			),
		},
	)
}

func keyFunc(key *rsa.PrivateKey) func(token *jwt.Token) (interface{}, error) {
	return func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return key.Public(), nil
	}
}
