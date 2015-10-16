package login

import (
	"net/http"

	"github.com/concourse/atc/web"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
)

type basicAuthHandler struct {
	logger lager.Logger
}

func NewBasicAuthHandler(
	logger lager.Logger,
) http.Handler {
	return &basicAuthHandler{
		logger: logger,
	}
}

func (handler *basicAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	redirect := r.FormValue("redirect")
	if redirect == "" {
		indexPath, err := web.Routes.CreatePathForRoute(web.Index, rata.Params{})
		if err != nil {
			handler.logger.Error("failed-to-generate-index-path", err)
		} else {
			redirect = indexPath
		}
	}

	http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
}
