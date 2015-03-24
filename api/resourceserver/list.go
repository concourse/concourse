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
		checkErr, err := s.resourceDB.GetResourceCheckError(resource.Name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resources = append(resources, present.Resource(resource, config.Groups, checkErr))
	}

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(resources)
}
