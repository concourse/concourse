package pipelineserver

import (
	"net/http"

	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/db"
)

type ScopedHandlerFactory struct {
}

func (f ScopedHandlerFactory) HandlerFor(pipelineScopedHandler func(db.Pipeline) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pipeline, ok := r.Context().Value(auth.PipelineContextKey).(db.Pipeline)
		if !ok {
			panic("missing pipeline")
		}

		pipelineScopedHandler(pipeline).ServeHTTP(w, r)
	}
}
