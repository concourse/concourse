package wallserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
)

func (s *Server) GetWall(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("wall")

	w.Header().Set("Content-Type", "application/json")

	wall, err := s.wall.GetWall()
	if err != nil {
		logger.Error("failed-to-get-message", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
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

	err = s.wall.SetWall(wall)
	if err != nil {
		logger.Error("failed-to-set-wall-message", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
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
