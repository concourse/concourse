package wallserver

import (
	"encoding/json"
	"github.com/concourse/concourse/atc"
	"net/http"
	"time"
)

func (s *Server) GetWall(w http.ResponseWriter, r *http.Request) {
	var wall atc.Wall

	logger := s.logger.Session("wall")

	w.Header().Set("Content-Type", "application/json")

	message, err := s.wall.GetMessage()
	if err != nil {
		logger.Error("failed-to-get-message", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	wall.Message = message

	expiration, err := s.wall.GetExpiration()
	if err != nil {
		logger.Error("failed-to-get-expiration", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !expiration.IsZero() {
		wall.ExpiresIn = time.Until(expiration).Round(time.Second).String()
	}

	err = json.NewEncoder(w).Encode(wall)
	if err != nil {
		logger.Error("failed-to-encode-json", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Server) SetWall(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("wall")

	var wall atc.Wall
	err := json.NewDecoder(r.Body).Decode(&wall)
	if err != nil {
		logger.Error("failed-to-decode-json", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = s.wall.SetMessage(wall.Message)
	if err != nil {
		logger.Error("failed-to-set-wall-message", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	expiresIn := wall.ExpiresIn
	if expiresIn != "" {
		duration, err := time.ParseDuration(expiresIn)
		if err != nil {
			logger.Error("failed-to-parse-expiration-duration", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = s.wall.SetExpiration(duration)
		if err != nil {
			logger.Error("failed-to-set-wall-expiration", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func (s *Server) ClearWall(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("wall")
	err := s.wall.Clear()
	if err != nil {
		logger.Error("failed-to-clear-the-wall", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
