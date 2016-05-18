package configserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
)

func (s *Server) GetConfig(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("get-config")
	pipelineName := rata.Param(r, "pipeline_name")
	teamDB := s.teamDBFactory.GetTeamDB(rata.Param(r, "team_name"))
	config, rawConfig, id, err := teamDB.GetConfig(pipelineName)
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
