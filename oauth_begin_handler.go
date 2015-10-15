package auth

import (
	"net/http"

	"github.com/pivotal-golang/lager"
)

type OAuthBeginHandler struct {
	logger    lager.Logger
	providers Providers
}

func NewOAuthBeginHandler(
	logger lager.Logger,
	providers Providers,
) http.Handler {
	return &OAuthBeginHandler{
		logger:    logger,
		providers: providers,
	}
}

func (handler *OAuthBeginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	provider, found := handler.providers[r.FormValue("provider")]
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	authCodeURL := provider.AuthCodeURL("bogus-state") // TODO
	http.Redirect(w, r, authCodeURL, http.StatusTemporaryRedirect)
}
