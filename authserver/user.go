package authserver

import (
	"encoding/json"
	"errors"
	"net/http"
)

type User struct {
	Team   *Team `json:"team,omitempty"`
	System *bool `json:"system,omitempty"`
}

type Team struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

func (s *Server) GetUser(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("user")

	w.Header().Set("Content-Type", "application/json")

	isAuthenticated := s.tokenValidator.IsAuthenticated(r)
	if !isAuthenticated {
		hLog.Error("not-authorized", errors.New("not-authorized"))
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	user, err := s.parseUser(r)
	if err != nil {
		hLog.Error("failed-to-get-team", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = json.NewEncoder(w).Encode(user)
	if err != nil {
		hLog.Error("failed-to-encode-user", errors.New("failed-to-get-team"))
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Server) parseUser(r *http.Request) (*User, error) {
	isSystem, found := s.tokenReader.GetSystem(r)
	if found {
		return &User{System: &isSystem}, nil
	}

	teamName, _, found := s.tokenReader.GetTeam(r)
	if !found {
		return nil, errors.New("failed-to-get-team-from-auth")
	}

	team, found, err := s.teamFactory.FindTeam(teamName)
	if err != nil {
		return nil, errors.New("failed-to-get-team-from-db")
	}

	if !found {
		return nil, errors.New("team-not-found-in-db")
	}

	return &User{Team: &Team{team.ID(), team.Name()}}, nil
}
