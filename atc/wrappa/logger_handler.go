package wrappa

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/auth"
)

type LoggerHandler struct {
	Logger  lager.Logger
	Handler http.Handler
}

func (handler LoggerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := handler.Logger.Session("http-request", lager.Data{
		"request-path": r.URL.Path,
	})
	ctx := context.WithValue(r.Context(), auth.LoggerContextKey, logger)
	handler.Handler.ServeHTTP(w, r.WithContext(ctx))
}
