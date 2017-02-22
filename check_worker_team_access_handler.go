package auth

import (
	"net/http"

	"github.com/concourse/atc/dbng"
)

type CheckWorkerTeamAccessHandlerFactory interface {
	HandlerFor(pipelineScopedHandler http.Handler, rejector Rejector) http.Handler
}

type checkWorkerTeamAccessHandlerFactory struct {
	workerFactory dbng.WorkerFactory
}

func NewCheckWorkerTeamAccessHandlerFactory(
	workerFactory dbng.WorkerFactory,
) CheckWorkerTeamAccessHandlerFactory {
	return &checkWorkerTeamAccessHandlerFactory{
		workerFactory: workerFactory,
	}
}

func (f *checkWorkerTeamAccessHandlerFactory) HandlerFor(
	delegateHandler http.Handler,
	rejector Rejector,
) http.Handler {
	return checkWorkerTeamHandler{
		rejector:        rejector,
		workerFactory:   f.workerFactory,
		delegateHandler: delegateHandler,
	}
}

type checkWorkerTeamHandler struct {
	rejector        Rejector
	workerFactory   dbng.WorkerFactory
	delegateHandler http.Handler
}

func (h checkWorkerTeamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !IsAuthenticated(r) {
		h.rejector.Unauthorized(w, r)
		return
	}

	if IsSystem(r) {
		h.delegateHandler.ServeHTTP(w, r)
		return
	}

	team, found := GetTeam(r)
	if !found {
		h.rejector.Unauthorized(w, r)
		return
	}

	if team.IsAdmin() {
		h.delegateHandler.ServeHTTP(w, r)
		return
	}

	workerName := r.FormValue(":worker_name")

	worker, found, err := h.workerFactory.GetWorker(workerName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if worker.TeamName() != team.Name() {
		h.rejector.Forbidden(w, r)
		return
	}

	h.delegateHandler.ServeHTTP(w, r)
}
