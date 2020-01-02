package wallserver

import (
	"encoding/json"
	"github.com/concourse/concourse/atc"
	"net/http"
	"time"
)

func (s *Server) GetExpiration(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("wall")

	w.Header().Set("Content-Type", "application/json")

	expiresAt, err := s.wall.GetExpiration()
	if err != nil {
		logger.Error("failed-to-get-expiration", err)
	}

	var wall atc.Wall
	if !expiresAt.IsZero() {
		wall.ExpiresIn = time.Until(expiresAt).Round(time.Second).String()
	}

	err = json.NewEncoder(w).Encode(wall)
	if err != nil {
		logger.Error("failed-to-encode-json", err)
	}
}

func (s *Server) SetExpiration(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("wall")

	var wall atc.Wall
	err := json.NewDecoder(r.Body).Decode(&wall)
	if err != nil {
		logger.Error("failed-to-decode-json", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	expiresIn, err := time.ParseDuration(wall.ExpiresIn)
	if err != nil {
		logger.Error("failed-to-parse-duration", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = s.wall.SetExpiration(expiresIn)
	if err != nil {
		logger.Error("failed-to-set-expiration", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}