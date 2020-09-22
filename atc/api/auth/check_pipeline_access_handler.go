package auth

import (
	"context"
	"net/http"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/db"
)

type CheckPipelineAccessHandlerFactory struct {
}

func (f *CheckPipelineAccessHandlerFactory) HandlerFor(
	delegateHandler http.Handler,
	rejector Rejector,
) http.Handler {
	return checkPipelineAccessHandler{
		rejector:        rejector,
		delegateHandler: delegateHandler,
	}
}

type checkPipelineAccessHandler struct {
	rejector        Rejector
	delegateHandler http.Handler
}

func (h checkPipelineAccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pipeline, ok := r.Context().Value(PipelineContextKey).(db.Pipeline)
	if !ok {
		panic("missing pipeline")
	}

	acc := accessor.GetAccessor(r)

	if acc.IsAuthorized(pipeline.TeamName()) || pipeline.Public() {
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
