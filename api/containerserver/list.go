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
	"github.com/concourse/atc/dbng"
)

func (s *Server) ListContainers(teamDB db.TeamDB, team dbng.Team) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		containers, err := team.FindContainersByMetadata(containerMetadata)
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

func (s *Server) parseRequest(r *http.Request) (dbng.ContainerMetadata, error) {
	var err error

	var containerType dbng.ContainerType
	if r.URL.Query().Get("type") != "" {
		containerType, err = dbng.ContainerTypeFromString(r.URL.Query().Get("type"))
		if err != nil {
			return dbng.ContainerMetadata{}, err
		}
	}

	pipelineID, err := s.parseIntParam(r, "pipeline_id")
	if err != nil {
		return dbng.ContainerMetadata{}, err
	}

	jobID, err := s.parseIntParam(r, "job_id")
	if err != nil {
		return dbng.ContainerMetadata{}, err
	}

	buildID, err := s.parseIntParam(r, "build_id")
	if err != nil {
		return dbng.ContainerMetadata{}, err
	}

	resourceID, err := s.parseIntParam(r, "resource_id")
	if err != nil {
		return dbng.ContainerMetadata{}, err
	}

	resourceTypeID, err := s.parseIntParam(r, "resource_type_id")
	if err != nil {
		return dbng.ContainerMetadata{}, err
	}

	query := r.URL.Query()

	return dbng.ContainerMetadata{
		Type: containerType,

		StepName: query.Get("step_name"),
		Attempt:  query.Get("attempt"),

		PipelineID:     pipelineID,
		JobID:          jobID,
		BuildID:        buildID,
		ResourceID:     resourceID,
		ResourceTypeID: resourceTypeID,

		PipelineName:     query.Get("pipeline_name"),
		JobName:          query.Get("job_name"),
		BuildName:        query.Get("build_name"),
		ResourceName:     query.Get("resource_name"),
		ResourceTypeName: query.Get("resource_type_name"),
	}, nil
}

func (s *Server) parseIntParam(r *http.Request, name string) (int, error) {
	var val int
	param := r.URL.Query().Get(name)
	if len(param) != 0 {
		var err error
		val, err = strconv.Atoi(param)
		if err != nil {
			return 0, fmt.Errorf("non-numeric '%s' param (%s): %s", name, param, err)
		}
	}

	return val, nil
}
