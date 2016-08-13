package containerserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) ListContainers(teamDB db.TeamDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.RawQuery
		hLog := s.logger.Session("list-containers", lager.Data{
			"params": params,
		})

		containerDescriptor, err := s.parseRequest(r)
		if err != nil {
			hLog.Error("failed-to-parse-request", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		hLog.Debug("listing-containers")

		containers, err := teamDB.FindContainersByDescriptors(containerDescriptor)
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
	})
}

func (s *Server) parseRequest(r *http.Request) (db.Container, error) {
	var containerType db.ContainerType
	var attempts []int
	var buildID int
	var err error
	if r.URL.Query().Get("type") != "" {
		containerType, err = db.ContainerTypeFromString(r.URL.Query().Get("type"))
		if err != nil {
			return db.Container{}, err
		}
	}

	if r.URL.Query().Get("attempt") != "" {
		err = json.Unmarshal([]byte(r.URL.Query().Get("attempt")), &attempts)
		if err != nil {
			return db.Container{}, err
		}
	}

	buildIDParam := r.URL.Query().Get("build-id")
	if len(buildIDParam) != 0 {
		var err error
		buildID, err = strconv.Atoi(buildIDParam)
		if err != nil {
			return db.Container{}, fmt.Errorf("malformed build ID: %s", err)
		}
	}

	container := db.Container{
		ContainerIdentifier: db.ContainerIdentifier{
			BuildID: buildID,
		},
		ContainerMetadata: db.ContainerMetadata{
			PipelineName: r.URL.Query().Get("pipeline_name"),
			JobName:      r.URL.Query().Get("job_name"),
			Type:         containerType,
			ResourceName: r.URL.Query().Get("resource_name"),
			StepName:     r.URL.Query().Get("step_name"),
			BuildName:    r.URL.Query().Get("build_name"),
			Attempts:     attempts,
		},
	}

	return container, nil
}
