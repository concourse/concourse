package pipelineserver

import (
	"net/http"

	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/db"
)

type RejectArchivedHandlerFactory struct {
}

func (f RejectArchivedHandlerFactory) RejectArchived(handler http.Handler) http.Handler {
	return RejectArchivedHandler{
		delegateHandler: handler,
	}
}

type RejectArchivedHandler struct {
	delegateHandler http.Handler
}

func (ra RejectArchivedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pipeline, ok := r.Context().Value(auth.PipelineContextKey).(db.Pipeline)
	if !ok {
		panic("missing pipeline")
	}

	if pipeline.Archived() {
		http.Error(w, "action not allowed for an archived pipeline", http.StatusConflict)
		return
	}

	ra.delegateHandler.ServeHTTP(w, r)
}
