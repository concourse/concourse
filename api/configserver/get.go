package configserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
)

func (s *Server) GetConfig(w http.ResponseWriter, r *http.Request) {
	pipelineName := rata.Param(r, "pipeline_name")
	config, id, err := s.db.GetConfig(pipelineName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set(atc.ConfigVersionHeader, fmt.Sprintf("%d", id))

	json.NewEncoder(w).Encode(config)
}
