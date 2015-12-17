package auth

import (
	"crypto/rsa"
	"fmt"
	"net/http"

	"github.com/concourse/atc/auth/provider"
	"github.com/dgrijalva/jwt-go"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
)

var SigningMethod = jwt.SigningMethodRS256

func NewOAuthHandler(
	logger lager.Logger,
	providers provider.Providers,
	signingKey *rsa.PrivateKey,
) (http.Handler, error) {
	return rata.NewRouter(OAuthRoutes, map[string]http.Handler{
		OAuthBegin: NewOAuthBeginHandler(
			logger.Session("oauth-begin"),
			providers,
			signingKey,
		),

		OAuthCallback: NewOAuthCallbackHandler(
			logger.Session("oauth-callback"),
			providers,
			signingKey,
		),
	})
}

func keyFunc(key *rsa.PrivateKey) func(token *jwt.Token) (interface{}, error) {
	return func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return key.Public(), nil
	}
}
