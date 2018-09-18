package infoserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/creds"
)

// Creds returns information on the credential manager attached to this instance of concourse.
// If no credential manager is configured the response will be empty.
// No actual credentials are shown in the response.
func (s *Server) Creds(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("creds")

	w.Header().Set("Content-Type", "application/json")

	configuredManagers := make(creds.Managers)

	for name, manager := range s.credsManagers {
		if manager.IsConfigured() {
			configuredManagers[name] = manager
		}
	}

	err := json.NewEncoder(w).Encode(configuredManagers)
	if err != nil {
		logger.Error("failed-to-encode-info", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
