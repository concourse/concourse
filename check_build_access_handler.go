package auth

import (
	"context"
	"net/http"
	"strconv"

	"github.com/concourse/atc/db"
)

const BuildKey = "build"

type CheckBuildAccessHandlerFactory interface {
	AnyJobHandler(delegateHandler http.Handler, rejector Rejector) http.Handler
	CheckIfPrivateJobHandler(delegateHandler http.Handler, rejector Rejector) http.Handler
}

//go:generate counterfeiter . BuildsDB

type BuildsDB interface {
	GetBuildByID(buildID int) (db.Build, bool, error)
}

type checkBuildAccessHandlerFactory struct {
	buildsDB BuildsDB
}

func NewCheckBuildAccessHandlerFactory(
	buildsDB BuildsDB,
) *checkBuildAccessHandlerFactory {
	return &checkBuildAccessHandlerFactory{
		buildsDB: buildsDB,
	}
}

func (f *checkBuildAccessHandlerFactory) AnyJobHandler(
	delegateHandler http.Handler,
	rejector Rejector,
) http.Handler {
	return checkBuildAccessHandler{
		rejector:        rejector,
		buildsDB:        f.buildsDB,
		delegateHandler: delegateHandler,
		allowPrivateJob: true,
	}
}

func (f *checkBuildAccessHandlerFactory) CheckIfPrivateJobHandler(
	delegateHandler http.Handler,
	rejector Rejector,
) http.Handler {
	return checkBuildAccessHandler{
		rejector:        rejector,
		buildsDB:        f.buildsDB,
		delegateHandler: delegateHandler,
		allowPrivateJob: false,
	}
}

type checkBuildAccessHandler struct {
	rejector        Rejector
	buildsDB        BuildsDB
	delegateHandler http.Handler
	allowPrivateJob bool
}

func (h checkBuildAccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
