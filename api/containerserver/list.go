package containerserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
)

func (s *Server) ListContainers(team db.Team) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.RawQuery
		hLog := s.logger.Session("list-containers", lager.Data{
			"params": params,
		})

		containerLocator, err := createContainerLocatorFromRequest(team, r, s.variablesFactory)
		if err != nil {
			hLog.Error("failed-to-parse-request", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		hLog.Debug("listing-containers")

		containers, err := containerLocator.Locate(hLog)
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

		err = json.NewEncoder(w).Encode(presentedContainers)
		if err != nil {
			hLog.Error("failed-to-encode-containers", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}

type containerLocator interface {
	Locate(logger lager.Logger) ([]db.Container, error)
}

func createContainerLocatorFromRequest(team db.Team, r *http.Request, variablesFactory creds.VariablesFactory) (containerLocator, error) {
	query := r.URL.Query()

	if query.Get("type") == "check" {
		return &checkContainerLocator{
			team:             team,
			pipelineName:     query.Get("pipeline_name"),
			resourceName:     query.Get("resource_name"),
			variablesFactory: variablesFactory,
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

type checkContainerLocator struct {
	team             db.Team
	pipelineName     string
	resourceName     string
	variablesFactory creds.VariablesFactory
}

func (l *checkContainerLocator) Locate(logger lager.Logger) ([]db.Container, error) {
	return l.team.FindCheckContainers(logger, l.pipelineName, l.resourceName, l.variablesFactory)
}

type stepContainerLocator struct {
	team     db.Team
	metadata db.ContainerMetadata
}

func (l *stepContainerLocator) Locate(logger lager.Logger) ([]db.Container, error) {
	return l.team.FindContainersByMetadata(l.metadata)
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
