package pipelineserver

import (
	"net/http"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/dbng"
)

type ScopedHandlerFactory struct {
	teamDBNGFactory dbng.TeamFactory
}

func NewScopedHandlerFactory(
	teamDBNGFactory dbng.TeamFactory,
) *ScopedHandlerFactory {
	return &ScopedHandlerFactory{
		teamDBNGFactory: teamDBNGFactory,
	}
}

func (pdbh *ScopedHandlerFactory) HandlerFor(pipelineScopedHandler func(dbng.Pipeline) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamName := r.FormValue(":team_name")
		pipelineName := r.FormValue(":pipeline_name")

		pipeline, ok := r.Context().Value(auth.PipelineContextKey).(dbng.Pipeline)
		if !ok {
			dbngTeam, found, err := pdbh.teamDBNGFactory.FindTeam(teamName)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !found {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			pipeline, found, err = dbngTeam.Pipeline(pipelineName)
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
