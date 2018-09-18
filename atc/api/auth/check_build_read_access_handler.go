package auth

import (
	"context"
	"net/http"
	"strconv"

	"github.com/concourse/atc/api/accessor"
	"github.com/concourse/atc/db"
)

type CheckBuildReadAccessHandlerFactory interface {
	AnyJobHandler(delegateHandler http.Handler, rejector Rejector) http.Handler
	CheckIfPrivateJobHandler(delegateHandler http.Handler, rejector Rejector) http.Handler
}

type checkBuildReadAccessHandlerFactory struct {
	buildFactory db.BuildFactory
}

func NewCheckBuildReadAccessHandlerFactory(
	buildFactory db.BuildFactory,
) *checkBuildReadAccessHandlerFactory {
	return &checkBuildReadAccessHandlerFactory{
		buildFactory: buildFactory,
	}
}

func (f *checkBuildReadAccessHandlerFactory) AnyJobHandler(
	delegateHandler http.Handler,
	rejector Rejector,
) http.Handler {
	return checkBuildReadAccessHandler{
		rejector:        rejector,
		buildFactory:    f.buildFactory,
		delegateHandler: delegateHandler,
		allowPrivateJob: true,
	}
}

func (f *checkBuildReadAccessHandlerFactory) CheckIfPrivateJobHandler(
	delegateHandler http.Handler,
	rejector Rejector,
) http.Handler {
	return checkBuildReadAccessHandler{
		rejector:        rejector,
		buildFactory:    f.buildFactory,
		delegateHandler: delegateHandler,
		allowPrivateJob: false,
	}
}

type checkBuildReadAccessHandler struct {
	rejector        Rejector
	buildFactory    db.BuildFactory
	delegateHandler http.Handler
	allowPrivateJob bool
}

func (h checkBuildReadAccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	buildIDStr := r.FormValue(":build_id")
	buildID, err := strconv.Atoi(buildIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	build, found, err := h.buildFactory.Build(buildID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	acc := accessor.GetAccessor(r)

	if !acc.IsAuthenticated() || !acc.IsAuthorized(build.TeamName()) {
		pipeline, found, err := build.Pipeline()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			h.rejector.Unauthorized(w, r)
			return
		}

		if !pipeline.Public() {
			if acc.IsAuthenticated() {
				h.rejector.Forbidden(w, r)
				return
			}

			h.rejector.Unauthorized(w, r)
			return
		}

		if !h.allowPrivateJob {
			job, found, err := pipeline.Job(build.JobName())
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !found {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			if !job.Config().Public {
				if acc.IsAuthenticated() {
					h.rejector.Forbidden(w, r)
					return
				}

				h.rejector.Unauthorized(w, r)
				return
			}
		}
	}

	ctx := context.WithValue(r.Context(), BuildContextKey, build)
	h.delegateHandler.ServeHTTP(w, r.WithContext(ctx))
}
