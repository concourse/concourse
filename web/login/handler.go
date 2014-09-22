package login

import (
	"net/http"

	"github.com/pivotal-golang/lager"
)

type handler struct {
	logger lager.Logger
}

func NewHandler(logger lager.Logger) http.Handler {
	return &handler{
		logger: logger,
	}
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	redirectPath := r.FormValue("redirect-to")
	if redirectPath == "" {
		redirectPath = "/"
	}

	http.Redirect(w, r, redirectPath, http.StatusFound)
}
