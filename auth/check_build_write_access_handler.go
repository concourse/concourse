package auth

import (
	"context"
	"net/http"
	"strconv"
)

type CheckBuildWriteAccessHandlerFactory interface {
	HandlerFor(delegateHandler http.Handler, rejector Rejector) http.Handler
}

type checkBuildWriteAccessHandlerFactory struct {
	buildsDB BuildsDB
}

func NewCheckBuildWriteAccessHandlerFactory(
	buildsDB BuildsDB,
) *checkBuildWriteAccessHandlerFactory {
	return &checkBuildWriteAccessHandlerFactory{
		buildsDB: buildsDB,
	}
}

func (f *checkBuildWriteAccessHandlerFactory) HandlerFor(
	delegateHandler http.Handler,
	rejector Rejector,
) http.Handler {
	return checkBuildWriteAccessHandler{
		rejector:        rejector,
		buildsDB:        f.buildsDB,
		delegateHandler: delegateHandler,
	}
}

type checkBuildWriteAccessHandler struct {
	rejector        Rejector
	buildsDB        BuildsDB
	delegateHandler http.Handler
}

func (h checkBuildWriteAccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !IsAuthenticated(r) {
		h.rejector.Unauthorized(w, r)
		return
	}

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
	if teamFound && teamName != build.TeamName() {
		h.rejector.Forbidden(w, r)
		return
	}

	ctx := context.WithValue(r.Context(), BuildKey, build)
	h.delegateHandler.ServeHTTP(w, r.WithContext(ctx))
}
