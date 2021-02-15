package infoserver

import (
	"encoding/json"
	"net/http"
)

// Creds returns information on the credential manager attached to this instance of concourse.
// If no credential manager is configured the response will be empty.
// No actual credentials are shown in the response.
func (s *Server) Creds(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("creds")

	w.Header().Set("Content-Type", "application/json")

	var credManager map[string]interface{}
	if s.credsManager != nil {
		credManager = map[string]interface{}{s.credsManager.Name(): s.credsManager.Config()}
	}

	err := json.NewEncoder(w).Encode(credManager)
	if err != nil {
		logger.Error("failed-to-encode-info", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
