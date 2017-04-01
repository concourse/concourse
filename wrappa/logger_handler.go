package wrappa

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/lager"
)

type LoggerHandler struct {
	Logger  lager.Logger
	Handler http.Handler
}

func (handler LoggerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := handler.Logger.Session("http-request", lager.Data{
		"request-path": r.URL.Path,
	})
	ctx := context.WithValue(r.Context(), "logger", logger)
	handler.Handler.ServeHTTP(w, r.WithContext(ctx))
}
