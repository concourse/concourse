package containerserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

func (s *Server) ListContainers(w http.ResponseWriter, r *http.Request) {
	params := r.URL.RawQuery
	hLog := s.logger.Session("hijack", lager.Data{
		"params": params,
	})

	workerIdentifier, err := s.parseRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var errors []string
	containers, err := s.workerClient.FindContainersForIdentifier(workerIdentifier)
	if err != nil {
		// If MulitWorkerError then iterate over each error and append workerName: error: etc
		if mwe, ok := err.(worker.MultiWorkerError); ok {
			for workerName, workerErr := range mwe.Errors() {
				errors = append(errors, fmt.Sprintf("workerName: %s, error: %s",
					workerName, workerErr.Error()))
			}
		} else {
			errors = make([]string, 1)
			errors[0] = err.Error()
		}
		hLog.Info("failed-to-get-container", lager.Data{"error": err})
		w.WriteHeader(http.StatusInternalServerError)
	}

	presentedContainers := make([]atc.Container, len(containers))
	for i := 0; i < len(containers); i++ {
		container := containers[i]
		presentedContainers[i] = present.Container(container)
		container.Release()
	}

	returnBody := atc.ListContainersReturn{
		Containers: presentedContainers,
		Errors:     errors,
	}
	json.NewEncoder(w).Encode(returnBody)
}

func (s *Server) parseRequest(r *http.Request) (worker.Identifier, error) {
	workerIdentifier := worker.Identifier{
		PipelineName: r.URL.Query().Get("pipeline_name"),
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
