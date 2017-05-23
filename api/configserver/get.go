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

	config, rawConfig, id, err := pipeline.Config()
	if err != nil {
		if malformedErr, ok := err.(atc.MalformedConfigError); ok {
			getConfigResponse := atc.ConfigResponse{
				Errors:    []string{malformedErr.Error()},
				RawConfig: rawConfig,
			}

			responseJSON, err := json.Marshal(getConfigResponse)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}

			w.Header().Set(atc.ConfigVersionHeader, fmt.Sprintf("%d", id))
			w.Write(responseJSON)

			return
		}

		logger.Error("failed-to-get-config", err)
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	w.Header().Set(atc.ConfigVersionHeader, fmt.Sprintf("%d", id))

	json.NewEncoder(w).Encode(atc.ConfigResponse{
		Config:    &config,
		RawConfig: rawConfig,
	})
}
