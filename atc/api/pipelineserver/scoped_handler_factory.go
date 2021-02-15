package pipelineserver

import (
	"net/http"

	"github.com/concourse/concourse/atc"
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
		teamName := r.FormValue(":team_name")
		pipelineName := r.FormValue(":pipeline_name")
		pipelineRef := atc.PipelineRef{Name: pipelineName}
		var err error
		pipelineRef.InstanceVars, err = atc.InstanceVarsFromQueryParams(r.URL.Query())
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

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

			pipeline, found, err = dbTeam.Pipeline(pipelineRef)
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
