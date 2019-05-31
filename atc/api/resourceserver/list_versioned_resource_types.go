package resourceserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/v5/atc/api/accessor"
	"github.com/concourse/concourse/v5/atc/api/present"
	"github.com/concourse/concourse/v5/atc/db"
)

func (s *Server) ListVersionedResourceTypes(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("list-versioned-resource-types")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceTypes, err := pipeline.ResourceTypes()
		if err != nil {
			logger.Error("failed-to-get-resources-types", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		acc := accessor.GetAccessor(r)
		showCheckErr := acc.IsAuthenticated()
		versionedResourceTypes := present.VersionedResourceTypes(showCheckErr, resourceTypes)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(versionedResourceTypes)
		if err != nil {
			logger.Error("failed-to-encode-versioned-resource-types", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
