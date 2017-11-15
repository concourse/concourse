package configserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
)

func (s *Server) GetConfig(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("get-config")
	pipelineName := rata.Param(r, "pipeline_name")
	teamName := rata.Param(r, "team_name")

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

	pipeline, found, err := team.Pipeline(pipelineName)
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

	jobs, err := pipeline.Jobs()
	if err != nil {
		logger.Error("failed-to-get-jobs", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resources, err := pipeline.Resources()
	if err != nil {
		logger.Error("failed-to-get-resources", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resourceTypes, err := pipeline.ResourceTypes()
	if err != nil {
		logger.Error("failed-to-get-resourceTypes", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	config := atc.Config{
		Groups:        pipeline.Groups(),
		Resources:     resources.Configs(),
		ResourceTypes: resourceTypes.Configs(),
		Jobs:          jobs.Configs(),
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		logger.Error("failed-to-marshal-config", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set(atc.ConfigVersionHeader, fmt.Sprintf("%d", pipeline.ConfigVersion()))

	err = json.NewEncoder(w).Encode(atc.ConfigResponse{
		Config:    &config,
		RawConfig: atc.RawConfig(rawConfig),
	})
	if err != nil {
		logger.Error("failed-to-encode-config", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
