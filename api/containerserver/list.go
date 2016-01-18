package containerserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

func (s *Server) ListContainers(w http.ResponseWriter, r *http.Request) {
	params := r.URL.RawQuery

	hLog := s.logger.Session("list-containers", lager.Data{
		"params": params,
	})

	containerMetadata, err := s.parseRequest(r)
	if err != nil {
		hLog.Error("failed-to-parse-request", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	hLog.Debug("listing-containers")

	containers, err := s.db.FindContainersByMetadata(containerMetadata)
	if err != nil {
		hLog.Error("failed-to-find-containers", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	hLog.Debug("listed", lager.Data{"container-count": len(containers)})

	presentedContainers := make([]atc.Container, len(containers))
	for i := 0; i < len(containers); i++ {
		container := containers[i]
		presentedContainers[i] = present.Container(container)
	}

	json.NewEncoder(w).Encode(presentedContainers)
}

func (s *Server) parseRequest(r *http.Request) (db.ContainerMetadata, error) {
	var containerType db.ContainerType
	var attempts []int
	var err error
	if r.URL.Query().Get("type") != "" {
		containerType, err = db.ContainerTypeFromString(r.URL.Query().Get("type"))
		if err != nil {
			return db.ContainerMetadata{}, err
		}
	}

	if r.URL.Query().Get("attempts") != "" {
		attempts, err = db.AttemptsSliceFromString(r.URL.Query().Get("attempts"))
		if err != nil {
			return db.ContainerMetadata{}, err
		}
	}

	containerMetadata := db.ContainerMetadata{
		PipelineName: r.URL.Query().Get("pipeline_name"),
		JobName:      r.URL.Query().Get("job_name"),
		Type:         containerType,
		ResourceName: r.URL.Query().Get("resource_name"),
		StepName:     r.URL.Query().Get("step_name"),
		BuildName:    r.URL.Query().Get("build_name"),
		Attempts:     attempts,
	}

	buildIDParam := r.URL.Query().Get("build-id")

	if len(buildIDParam) != 0 {
		var err error
		containerMetadata.BuildID, err = strconv.Atoi(buildIDParam)
		if err != nil {
			return db.ContainerMetadata{}, fmt.Errorf("malformed build ID: %s", err)
		}
	}
	return containerMetadata, nil
}
