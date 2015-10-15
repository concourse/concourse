package auth

import (
	"crypto/rsa"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/pivotal-golang/lager"
)

type OAuthBeginHandler struct {
	logger     lager.Logger
	providers  Providers
	privateKey *rsa.PrivateKey
}

func NewOAuthBeginHandler(
	logger lager.Logger,
	providers Providers,
	privateKey *rsa.PrivateKey,
) http.Handler {
	return &OAuthBeginHandler{
		logger:     logger,
		providers:  providers,
		privateKey: privateKey,
	}
}

func (handler *OAuthBeginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	providerName := r.FormValue(":provider")

	provider, found := handler.providers[providerName]
	if !found {
		handler.logger.Info("unknown-provider", lager.Data{
			"provider": providerName,
		})

		w.WriteHeader(http.StatusNotFound)
		return
	}

	token := jwt.New(SigningMethod)
	token.Claims["exp"] = time.Now().Add(time.Hour).Unix()
	token.Claims["redirect"] = r.FormValue("redirect")

	signedState, err := token.SignedString(handler.privateKey)
	if err != nil {
		handler.logger.Error("failed-to-sign-state-string", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	authCodeURL := provider.AuthCodeURL(signedState)

	http.Redirect(w, r, authCodeURL, http.StatusTemporaryRedirect)
}
