package configserver

import (
	"encoding/json"
	"net/http"
)

func (s *Server) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := s.db.GetConfig()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(config)
}
