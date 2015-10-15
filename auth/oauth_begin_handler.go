package auth

import (
	"crypto/rsa"
	"net/http"

	"github.com/pivotal-golang/lager"
	"golang.org/x/oauth2"
)

type OAuthBeginHandler struct {
	Config *oauth2.Config
	Logger lager.Logger
	Key    *rsa.PrivateKey
}

func (handler *OAuthBeginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler.Config == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	authCodeURL := handler.Config.AuthCodeURL("bogus-state") // TODO
	http.Redirect(w, r, authCodeURL, http.StatusTemporaryRedirect)
}
