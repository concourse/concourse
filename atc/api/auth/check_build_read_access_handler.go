package auth

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/db"
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

	allow, err := h.allow(build, acc)
	if err != nil {
		if err == errDisappeared {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if allow {
		ctx := context.WithValue(r.Context(), BuildContextKey, build)
		h.delegateHandler.ServeHTTP(w, r.WithContext(ctx))
	} else if acc.IsAuthenticated() {
		h.rejector.Forbidden(w, r)
	} else {
		h.rejector.Unauthorized(w, r)
	}
}

// this is mainly to avoid a monstrosity like bool, bool, error; it's handled
// above
var errDisappeared = errors.New("internal: build parent disappeared")

func (h checkBuildReadAccessHandler) allow(build db.Build, acc accessor.Access) (bool, error) {
	if acc.IsAuthenticated() {
		allTeams := build.AllAssociatedTeamNames()
		authorized := false
		for _, team := range allTeams {
			if acc.IsAuthorized(team) {
				authorized = true
				break
			}
		}
		if authorized {
			return true, nil
		}
	}

	if build.PipelineID() == 0 {
		return false, nil
	}

	pipeline, found, err := build.Pipeline()
	if err != nil {
		return false, err
	}

	if !found {
		return false, errDisappeared
	}

	if !pipeline.Public() {
		return false, nil
	}

	if h.allowPrivateJob {
		return true, nil
	}

	if build.JobID() == 0 {
		return false, nil
	}

	job, found, err := pipeline.Job(build.JobName())
	if err != nil {
		return false, err
	}

	if !found {
		return false, errDisappeared
	}

	if job.Public() {
		return true, nil
	}

	return false, nil
}
