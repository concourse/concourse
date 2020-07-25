package configserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc"
	"github.com/tedsuo/rata"
)

func (s *Server) GetConfig(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("get-config")
	teamName := rata.Param(r, "team_name")
	pipelineName := rata.Param(r, "pipeline_name")
	pipelineRef := atc.PipelineRef{Name: pipelineName}
	if instanceVars := r.URL.Query().Get("instance_vars"); instanceVars != "" {
		err := json.Unmarshal([]byte(instanceVars), &pipelineRef.InstanceVars)
		if err != nil {
			logger.Error("malformed-instance-vars", err)
			s.handleBadRequest(w, fmt.Sprintf("instance_vars is malformed: %s", err))
			return
		}
	}

	team, found, err := s.teamFactory.FindTeam(teamName)
	if err != nil {
		logger.Error("failed-to-find-team", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		logger.Debug("team-not-found", lager.Data{"team": teamName})
		w.WriteHeader(http.StatusNotFound)
		return
	}

	pipeline, found, err := team.Pipeline(pipelineRef)
	if err != nil {
		logger.Error("failed-to-find-pipeline", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		logger.Debug("pipeline-not-found", lager.Data{"pipeline": pipelineName})
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if pipeline.Archived() {
		logger.Debug("pipeline-is-archived", lager.Data{"pipeline": pipelineName})
		w.WriteHeader(http.StatusNotFound)
		return
	}

	config, err := pipeline.Config()
	if err != nil {
		logger.Error("failed-to-get-pipeline-config", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set(atc.ConfigVersionHeader, fmt.Sprintf("%d", pipeline.ConfigVersion()))
	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(atc.ConfigResponse{
		Config: config,
	})
	if err != nil {
		logger.Error("failed-to-encode-config", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
