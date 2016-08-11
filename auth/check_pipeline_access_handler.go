package auth

import (
	"context"
	"net/http"

	"github.com/concourse/atc/db"
)

const PipelineDBKey = "pipelineDB"

type CheckPipelineAccessHandlerFactory interface {
	HandlerFor(pipelineScopedHandler http.Handler, rejector Rejector) http.Handler
}

type checkPipelineAccessHandlerFactory struct {
	pipelineDBFactory db.PipelineDBFactory
	teamDBFactory     db.TeamDBFactory
}

func NewCheckPipelineAccessHandlerFactory(
	pipelineDBFactory db.PipelineDBFactory,
	teamDBFactory db.TeamDBFactory,
) *checkPipelineAccessHandlerFactory {
	return &checkPipelineAccessHandlerFactory{
		pipelineDBFactory: pipelineDBFactory,
		teamDBFactory:     teamDBFactory,
	}
}

func (f *checkPipelineAccessHandlerFactory) HandlerFor(
	delegateHandler http.Handler,
	rejector Rejector,
) http.Handler {
	return checkPipelineAccessHandler{
		rejector:          rejector,
		teamDBFactory:     f.teamDBFactory,
		pipelineDBFactory: f.pipelineDBFactory,
		delegateHandler:   delegateHandler,
	}
}

type checkPipelineAccessHandler struct {
	rejector          Rejector
	teamDBFactory     db.TeamDBFactory
	pipelineDBFactory db.PipelineDBFactory
	delegateHandler   http.Handler
}

func (h checkPipelineAccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pipelineName := r.FormValue(":pipeline_name")
	requestTeamName := r.FormValue(":team_name")

	teamDB := h.teamDBFactory.GetTeamDB(requestTeamName)
	savedPipeline, found, err := teamDB.GetPipelineByName(pipelineName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	pipelineDB := h.pipelineDBFactory.Build(savedPipeline)
	if IsAuthorized(r) || pipelineDB.IsPublic() {
		ctx := context.WithValue(r.Context(), PipelineDBKey, pipelineDB)
		h.delegateHandler.ServeHTTP(w, r.WithContext(ctx))
		return
	}

	if IsAuthenticated(r) {
		h.rejector.Forbidden(w, r)
		return
	}

	h.rejector.Unauthorized(w, r)
}
