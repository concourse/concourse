package versionserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/db"
)

// IMPORTANT: This is not yet tested because it is not being used
func (s *Server) GetCausality(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		versionID, err := strconv.Atoi(r.FormValue(":resource_version_id"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		hLog := s.logger.Session("causality", lager.Data{
			"version": versionID,
		})

		causality, err := pipeline.Causality(versionID)
		if err != nil {
			hLog.Error("failed-to-fetch", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		hLog.Debug("fetched", lager.Data{"length": len(causality)})

		w.Header().Set("Content-Type", "application/json")

		w.WriteHeader(http.StatusOK)

		_ = json.NewEncoder(w).Encode(causality)
	})
}
