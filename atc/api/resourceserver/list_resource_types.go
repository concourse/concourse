package resourceserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ListResourceTypes(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("list-versioned-resource-types")

		resourceTypes, err := pipeline.ResourceTypes()
		if err != nil {
			logger.Error("failed-to-get-resources-types", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		presentResourceTypes := present.ResourceTypes(resourceTypes)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(presentResourceTypes)
		if err != nil {
			logger.Error("failed-to-encode-versioned-resource-types", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
