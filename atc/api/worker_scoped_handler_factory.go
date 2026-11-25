package api

import (
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/v8/atc/db"
)

type WorkerScopedHandlerFactory struct {
	logger        lager.Logger
	workerFactory db.WorkerFactory
}

func NewWorkerScopedHandlerFactory(
	logger lager.Logger,
	workerFactory db.WorkerFactory,
) *WorkerScopedHandlerFactory {
	return &WorkerScopedHandlerFactory{
		logger:        logger,
		workerFactory: workerFactory,
	}
}

func (ws *WorkerScopedHandlerFactory) HandlerFor(workerScopedHandler func(db.Worker) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := ws.logger.Session("worker-scoped-handler")

		workerName := r.FormValue(":worker_name")
		if workerName == "" {
			workerName = r.URL.Query().Get("worker_name")
		}

		if workerName == "" {
			logger.Error("empty-worker-name", nil)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		logger = logger.WithData(lager.Data{"worker": workerName})

		worker, found, err := ws.workerFactory.GetWorker(workerName)
		if err != nil {
			logger.Error("get-worker", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Error("worker-not-found", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		teamName := r.URL.Query().Get("team_name")
		if teamName != worker.TeamName() {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		workerScopedHandler(worker).ServeHTTP(w, r)
	}
}
