package healthserver

import (
	"context"
	"encoding/json"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
	"net/http"
	"time"
)

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("health")

	w.Header().Set("Content-Type", "application/json")

	atcWorkers, err := s.listWorkers(r)
	if err != nil {
		logger.Error("failed-to-list-workers", err)
		json.NewEncoder(w).Encode(atc.Health{
			DBStatus: "NOK"})
		return
	}

	workersStatus := make(map[string]string, len(atcWorkers))
	for i := range atcWorkers {
		err = s.checkHealth(atcWorkers[i])
		if err != nil {
			logger.Error("failed-to-healthcheck-worker-"+atcWorkers[i].Name, err)
			workersStatus[atcWorkers[i].Name] = "NOK"
		} else {
			workersStatus[atcWorkers[i].Name] = "OK"
		}
	}

	err = json.NewEncoder(w).Encode(atc.Health{
		DBStatus:      "OK",
		WorkersStatus: workersStatus})

	if err != nil {
		logger.Error("failed-to-encode-health", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Server) listWorkers(r *http.Request) (atcWorkers []atc.Worker, err error) {
	var workers []db.Worker

	acc := accessor.GetAccessor(r)

	if acc.IsAdmin() {
		workers, err = s.dbWorkerFactory.Workers()
	} else {
		workers, err = s.dbWorkerFactory.VisibleWorkers(acc.TeamNames())
	}

	if err != nil {
		return nil, err
	}

	atcWorkers = make([]atc.Worker, len(workers))
	for i, savedWorker := range workers {
		atcWorkers[i] = present.Worker(savedWorker)
	}

	return atcWorkers, nil
}

func (s *Server) checkHealth(atcWorker atc.Worker) (err error) {
	//TODO hardcoded 10s
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()

	return s.doRequest(ctx, atcWorker.HealthcheckURL)
}

func (s *Server) doRequest(ctx context.Context, url string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	_ = resp.Body.Close()
	return nil
}
