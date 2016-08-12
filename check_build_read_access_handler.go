package auth

import (
	"context"
	"net/http"
	"strconv"
)

type CheckBuildReadAccessHandlerFactory interface {
	AnyJobHandler(delegateHandler http.Handler, rejector Rejector) http.Handler
	CheckIfPrivateJobHandler(delegateHandler http.Handler, rejector Rejector) http.Handler
}

type checkBuildReadAccessHandlerFactory struct {
	buildsDB BuildsDB
}

func NewCheckBuildReadAccessHandlerFactory(
	buildsDB BuildsDB,
) *checkBuildReadAccessHandlerFactory {
	return &checkBuildReadAccessHandlerFactory{
		buildsDB: buildsDB,
	}
}

func (f *checkBuildReadAccessHandlerFactory) AnyJobHandler(
	delegateHandler http.Handler,
	rejector Rejector,
) http.Handler {
	return checkBuildReadAccessHandler{
		rejector:        rejector,
		buildsDB:        f.buildsDB,
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
		buildsDB:        f.buildsDB,
		delegateHandler: delegateHandler,
		allowPrivateJob: false,
	}
}

type checkBuildReadAccessHandler struct {
	rejector        Rejector
	buildsDB        BuildsDB
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

	build, found, err := h.buildsDB.GetBuildByID(buildID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	teamName, _, _, teamFound := GetTeam(r)
	if !IsAuthenticated(r) || (teamFound && teamName != build.TeamName()) {
		if build.IsOneOff() {
			h.rejector.Unauthorized(w, r)
			return
		}

		pipeline, err := build.GetPipeline()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !pipeline.Public {
			if IsAuthenticated(r) {
				h.rejector.Forbidden(w, r)
				return
			}

			h.rejector.Unauthorized(w, r)
			return
		}

		if !h.allowPrivateJob {
			config, _, err := build.GetConfig()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			isJobPublic, err := config.JobIsPublic(build.JobName())
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !isJobPublic {
				if IsAuthenticated(r) {
					h.rejector.Forbidden(w, r)
					return
				}

				h.rejector.Unauthorized(w, r)
				return
			}
		}
	}

	ctx := context.WithValue(r.Context(), BuildKey, build)
	h.delegateHandler.ServeHTTP(w, r.WithContext(ctx))
}
