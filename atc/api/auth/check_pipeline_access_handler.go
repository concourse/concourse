package auth

import (
	"context"
	"net/http"

	"github.com/concourse/atc/api/accessor"
	"github.com/concourse/atc/db"
)

type CheckPipelineAccessHandlerFactory interface {
	HandlerFor(pipelineScopedHandler http.Handler, rejector Rejector) http.Handler
}

type checkPipelineAccessHandlerFactory struct {
	teamFactory db.TeamFactory
}

func NewCheckPipelineAccessHandlerFactory(
	teamFactory db.TeamFactory,
) *checkPipelineAccessHandlerFactory {
	return &checkPipelineAccessHandlerFactory{
		teamFactory: teamFactory,
	}
}

func (f *checkPipelineAccessHandlerFactory) HandlerFor(
	delegateHandler http.Handler,
	rejector Rejector,
) http.Handler {
	return checkPipelineAccessHandler{
		rejector:        rejector,
		teamFactory:     f.teamFactory,
		delegateHandler: delegateHandler,
	}
}

type checkPipelineAccessHandler struct {
	rejector        Rejector
	teamFactory     db.TeamFactory
	delegateHandler http.Handler
}

func (h checkPipelineAccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	teamName := r.FormValue(":team_name")
	pipelineName := r.FormValue(":pipeline_name")

	team, found, err := h.teamFactory.FindTeam(teamName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	pipeline, found, err := team.Pipeline(pipelineName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	acc := accessor.GetAccessor(r)

	if acc.IsAuthorized(teamName) || pipeline.Public() {
		ctx := context.WithValue(r.Context(), PipelineContextKey, pipeline)
		h.delegateHandler.ServeHTTP(w, r.WithContext(ctx))
		return
	}

	if !acc.IsAuthenticated() {
		h.rejector.Unauthorized(w, r)
		return
	}

	h.rejector.Forbidden(w, r)
}
