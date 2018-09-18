package resourceserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/accessor"
	"github.com/concourse/atc/api/present"
)

func (s *Server) ListAllResources(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-all-resources")

	acc := accessor.GetAccessor(r)

	dbResources, err := s.resourceFactory.VisibleResources(acc.TeamNames())
	if err != nil {
		logger.Error("failed-to-get-all-visible-resources", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var resources []atc.Resource

	for _, resource := range dbResources {
		resources = append(
			resources,
			present.Resource(
				resource,
				true,
				resource.TeamName(),
			),
		)
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(resources)
	if err != nil {
		logger.Error("failed-to-encode-resources", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
