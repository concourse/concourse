package healthserver

import (
	"encoding/json"
	"net/http"
)

// Creds returns information on the credential manager attached to this instance of concourse.
// If no credential manager is configured the response will be empty.
// No actual credentials are shown in the response.
func (s *Server) Creds(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("creds")
	randomStuff := []string{"this", "is", "your", "creds"}

	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(randomStuff)
	if err != nil {
		logger.Error("failed-to-encode-info", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
