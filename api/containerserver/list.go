package containerserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/worker"
)

func (s *Server) ListContainers(w http.ResponseWriter, r *http.Request) {
	workerIdentifier, err := s.parseRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	containers, _ := s.workerClient.LookupContainers(workerIdentifier)

	presentedContainers := make([]present.PresentedContainer, len(containers))
	for i := 0; i < len(containers); i++ {
		container := containers[i]
		presentedContainers[i] = present.Container(container)
		container.Release()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(presentedContainers)
}

func (s *Server) parseRequest(r *http.Request) (worker.Identifier, error) {
	workerIdentifier := worker.Identifier{
		PipelineName: r.URL.Query().Get("pipeline"),
		Type:         worker.ContainerType(r.URL.Query().Get("type")),
		Name:         r.URL.Query().Get("name"),
	}

	buildIDParam := r.URL.Query().Get("build-id")

	if len(buildIDParam) != 0 {
		var err error
		workerIdentifier.BuildID, err = strconv.Atoi(buildIDParam)
		if err != nil {
			return worker.Identifier{}, fmt.Errorf("malformed build ID: %s", err)
		}
	}
	return workerIdentifier, nil
}
