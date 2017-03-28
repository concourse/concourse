package pipelineserver

import (
	"net/http"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

type ScopedHandlerFactory struct {
	pipelineDBFactory db.PipelineDBFactory
	teamDBFactory     db.TeamDBFactory
	teamDBNGFactory   dbng.TeamFactory
}

func NewScopedHandlerFactory(
	pipelineDBFactory db.PipelineDBFactory,
	teamDBFactory db.TeamDBFactory,
	teamDBNGFactory dbng.TeamFactory,
) *ScopedHandlerFactory {
	return &ScopedHandlerFactory{
		pipelineDBFactory: pipelineDBFactory,
		teamDBFactory:     teamDBFactory,
		teamDBNGFactory:   teamDBNGFactory,
	}
}

func (pdbh *ScopedHandlerFactory) HandlerFor(pipelineScopedHandler func(db.PipelineDB, dbng.Pipeline) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamName := r.FormValue(":team_name")
		pipelineName := r.FormValue(":pipeline_name")

		pipelineDB, ok := r.Context().Value(auth.PipelineDBKey).(db.PipelineDB)
		if !ok {
			teamDB := pdbh.teamDBFactory.GetTeamDB(teamName)
			savedPipeline, found, err := teamDB.GetPipelineByName(pipelineName)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !found {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			pipelineDB = pdbh.pipelineDBFactory.Build(savedPipeline)
		}

		dbngTeam := pdbh.teamDBNGFactory.GetByID(pipelineDB.TeamID())

		dbPipeline, found, err := dbngTeam.FindPipelineByName(pipelineName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		pipelineScopedHandler(pipelineDB, dbPipeline).ServeHTTP(w, r)
	}
}
