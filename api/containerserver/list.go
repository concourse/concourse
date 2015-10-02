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

	hLog.Info("listing containers")

	containerIdentifier, err := s.parseRequest(r)
	if err != nil {
		hLog.Error("failed-to-parse-request", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	containers, err := s.db.FindContainerInfosByIdentifier(containerIdentifier)
	if err != nil {
		hLog.Error("failed-to-find-containers", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	presentedContainers := make([]atc.Container, len(containers))
	for i := 0; i < len(containers); i++ {
		container := containers[i]
		presentedContainers[i] = present.Container(container)
	}

	hLog.Info("found-containers", lager.Data{"containers": presentedContainers})

	json.NewEncoder(w).Encode(presentedContainers)
}

func (s *Server) parseRequest(r *http.Request) (db.ContainerIdentifier, error) {
	containerIdentifier := db.ContainerIdentifier{
		PipelineName: r.URL.Query().Get("pipeline_name"),
		Type:         db.ContainerType(r.URL.Query().Get("type")),
		Name:         r.URL.Query().Get("name"),
	}

	buildIDParam := r.URL.Query().Get("build-id")

	if len(buildIDParam) != 0 {
		var err error
		containerIdentifier.BuildID, err = strconv.Atoi(buildIDParam)
		if err != nil {
			return db.ContainerIdentifier{}, fmt.Errorf("malformed build ID: %s", err)
		}
	}
	return containerIdentifier, nil
}
