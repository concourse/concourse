package buildserver

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/api/auth"
	"github.com/concourse/atc/db"
)

type scopedHandlerFactory struct {
	logger lager.Logger
}

func NewScopedHandlerFactory(
	logger lager.Logger,
) *scopedHandlerFactory {
	return &scopedHandlerFactory{
		logger: logger,
	}
}

func (f *scopedHandlerFactory) HandlerFor(buildScopedHandler func(db.Build) http.Handler) http.HandlerFunc {
	logger := f.logger.Session("scoped-build-factory")

	return func(w http.ResponseWriter, r *http.Request) {
		build, ok := r.Context().Value(auth.BuildContextKey).(db.Build)
		if !ok {
			logger.Error("build-is-not-in-context", errors.New("build-is-not-in-context"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		buildScopedHandler(build).ServeHTTP(w, r)
	}
}
