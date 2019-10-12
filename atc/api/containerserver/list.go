package containerserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ListContainers(team db.Team) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.RawQuery
		hLog := s.logger.Session("list-containers", lager.Data{
			"params": params,
		})

		containerLocator, err := createContainerLocatorFromRequest(team, r, s.secretManager)
		if err != nil {
			hLog.Error("failed-to-parse-request", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		hLog.Debug("listing-containers")

		containers, checkContainersExpiresAt, err := containerLocator.Locate(hLog)
		if err != nil {
			hLog.Error("failed-to-find-containers", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		hLog.Debug("listed", lager.Data{"container-count": len(containers)})

		presentedContainers := make([]atc.Container, len(containers))
		for i := 0; i < len(containers); i++ {
			container := containers[i]
			presentedContainers[i] = present.Container(container, checkContainersExpiresAt[container.ID()])
		}

		err = json.NewEncoder(w).Encode(presentedContainers)
		if err != nil {
			hLog.Error("failed-to-encode-containers", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}

type containerLocator interface {
	Locate(lager.Logger) ([]db.Container, map[int]time.Time, error)
}

func createContainerLocatorFromRequest(team db.Team, r *http.Request, secretManager creds.Secrets) (containerLocator, error) {
	query := r.URL.Query()
	delete(query, ":team_name")

	if len(query) == 0 {
		return &allContainersLocator{
			team: team,
		}, nil
	}

	if query.Get("type") == "check" {
		return &checkContainerLocator{
			team:          team,
			pipelineName:  query.Get("pipeline_name"),
			resourceName:  query.Get("resource_name"),
			secretManager: secretManager,
		}, nil
	}

	var err error
	var containerType db.ContainerType
	if query.Get("type") != "" {
		containerType, err = db.ContainerTypeFromString(query.Get("type"))
		if err != nil {
			return nil, err
		}
	}

	pipelineID, err := parseIntParam(r, "pipeline_id")
	if err != nil {
		return nil, err
	}

	jobID, err := parseIntParam(r, "job_id")
	if err != nil {
		return nil, err
	}

	buildID, err := parseIntParam(r, "build_id")
	if err != nil {
		return nil, err
	}

	return &stepContainerLocator{
		team: team,

		metadata: db.ContainerMetadata{
			Type: containerType,

			StepName: query.Get("step_name"),
			Attempt:  query.Get("attempt"),

			PipelineID: pipelineID,
			JobID:      jobID,
			BuildID:    buildID,

			PipelineName: query.Get("pipeline_name"),
			JobName:      query.Get("job_name"),
			BuildName:    query.Get("build_name"),
		},
	}, nil
}

type allContainersLocator struct {
	team db.Team
}

func (l *allContainersLocator) Locate(logger lager.Logger) ([]db.Container, map[int]time.Time, error) {
	containers, err := l.team.Containers()
	return containers, nil, err
}

type checkContainerLocator struct {
	team          db.Team
	pipelineName  string
	resourceName  string
	secretManager creds.Secrets
}

func (l *checkContainerLocator) Locate(logger lager.Logger) ([]db.Container, map[int]time.Time, error) {
	return l.team.FindCheckContainers(logger, l.pipelineName, l.resourceName, l.secretManager)
}

type stepContainerLocator struct {
	team     db.Team
	metadata db.ContainerMetadata
}

func (l *stepContainerLocator) Locate(logger lager.Logger) ([]db.Container, map[int]time.Time, error) {
	containers, err := l.team.FindContainersByMetadata(l.metadata)
	return containers, nil, err
}

func parseIntParam(r *http.Request, name string) (int, error) {
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
