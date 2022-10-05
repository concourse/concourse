package pipelineserver

import (
	"net/http"

	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/db"
)

type ScopedHandlerFactory struct {
	teamDBFactory db.TeamFactory
}

func NewScopedHandlerFactory(
	teamDBFactory db.TeamFactory,
) *ScopedHandlerFactory {
	return &ScopedHandlerFactory{
		teamDBFactory: teamDBFactory,
	}
}

func (pdbh *ScopedHandlerFactory) HandlerFor(pipelineScopedHandler func(db.Pipeline) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamName := r.URL.Query().Get(":team_name")
		pipelineName := r.URL.Query().Get(":pipeline_name")

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
