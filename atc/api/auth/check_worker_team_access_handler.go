package auth

import (
	"net/http"

	"github.com/concourse/concourse/v5/atc/api/accessor"
	"github.com/concourse/concourse/v5/atc/db"
)

type CheckWorkerTeamAccessHandlerFactory interface {
	HandlerFor(pipelineScopedHandler http.Handler, rejector Rejector) http.Handler
}

type checkWorkerTeamAccessHandlerFactory struct {
	workerFactory db.WorkerFactory
}

func NewCheckWorkerTeamAccessHandlerFactory(
	workerFactory db.WorkerFactory,
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
	workerFactory   db.WorkerFactory
	delegateHandler http.Handler
}

func (h checkWorkerTeamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	acc := accessor.GetAccessor(r)
	if !acc.IsAuthenticated() {
		h.rejector.Unauthorized(w, r)
		return
	}

	if acc.IsSystem() || acc.IsAdmin() {
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

	if worker.TeamName() != "" && acc.IsAuthorized(worker.TeamName()) {
		h.delegateHandler.ServeHTTP(w, r)
		return
	}

	h.rejector.Forbidden(w, r)
}
