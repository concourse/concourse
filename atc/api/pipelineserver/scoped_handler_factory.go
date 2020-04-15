package pipelineserver

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/db"
)

type ScopedHandlerFactory struct {
	logger        lager.Logger
	teamDBFactory db.TeamFactory
}

type pipelineScopedHandler func(db.Pipeline) http.Handler

func NewScopedHandlerFactory(
	logger lager.Logger,
	teamDBFactory db.TeamFactory,
) *ScopedHandlerFactory {
	return &ScopedHandlerFactory{
		logger:        logger,
		teamDBFactory: teamDBFactory,
	}
}

func (pdbh *ScopedHandlerFactory) RejectArchived(pipelineScopedHandler pipelineScopedHandler) pipelineScopedHandler {
	return func(pipeline db.Pipeline) http.Handler {
		if pipeline.Archived() {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				pdbh.logger.Debug("pipeline-is-archived")
				http.Error(w, "action not allowed for archived pipeline", http.StatusConflict)
			})
		}
		return pipelineScopedHandler(pipeline)
	}
}

func (pdbh *ScopedHandlerFactory) HandlerFor(pipelineScopedHandler pipelineScopedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamName := r.FormValue(":team_name")
		pipelineName := r.FormValue(":pipeline_name")

		pipeline, ok := r.Context().Value(auth.PipelineContextKey).(db.Pipeline)
		if !ok {
			dbTeam, found, err := pdbh.teamDBFactory.FindTeam(teamName)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !found {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			pipeline, found, err = dbTeam.Pipeline(pipelineName)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !found {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}

		pipelineScopedHandler(pipeline).ServeHTTP(w, r)
	}
}
