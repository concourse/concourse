package resourceserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
)

func (s *Server) ListResources(w http.ResponseWriter, r *http.Request) {
	var resources []atc.Resource

	config, _, err := s.configDB.GetConfig()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, resource := range config.Resources {
		resources = append(resources, present.Resource(resource, config.Groups))
	}

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(resources)
}
