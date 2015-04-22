package configserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/atc"
)

func (s *Server) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, id, err := s.db.GetConfig()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set(atc.ConfigVersionHeader, fmt.Sprintf("%d", id))

	json.NewEncoder(w).Encode(config)
}
