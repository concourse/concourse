package authserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/concourse/atc/db"
	"github.com/concourse/skymarshal/provider"
	"github.com/google/jsonapi"
)

const BasicAuthDisplayName = "Basic Auth"

func (s *Server) ListAuthMethods(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-auth-methods")

	w.Header().Set("Content-Type", "application/json")

	teamName := r.FormValue("team_name")
	team, found, err := s.teamFactory.FindTeam(teamName)
	if err != nil {
		logger.Error("failed-to-get-team", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		logger.Info("team-not-found")
		w.WriteHeader(http.StatusNotFound)
		_ = jsonapi.MarshalErrors(w, []*jsonapi.ErrorObject{{
			Title:  "Team Not Found Error",
			Detail: fmt.Sprintf("Team with name '%s' not found.", teamName),
			Status: "404",
		}})
		return
	}

	methods, err := s.authMethods(team)
	if err != nil {
		logger.Error("failed-to-get-auth-methods", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	sort.Sort(byTypeAndName(methods))

	err = json.NewEncoder(w).Encode(methods)
	if err != nil {
		logger.Error("failed-to-encode-auth-methods", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Server) authMethods(team db.Team) ([]provider.AuthMethod, error) {
	methods := []provider.AuthMethod{}

	providers := provider.GetProviders()

	for providerName, config := range team.Auth() {
		p, found := providers[providerName]
		if !found {
			return nil, errors.New("failed to find provider")
		}

		authConfig, err := p.UnmarshalConfig(config)
		if err != nil {
			return nil, err
		}

		methods = append(methods, authConfig.AuthMethod(s.oAuthBaseURL, team.Name()))
	}

	if team.BasicAuth() != nil {
		methods = append(methods, provider.AuthMethod{
			Type:        provider.AuthTypeBasic,
			DisplayName: BasicAuthDisplayName,
			AuthURL:     s.externalURL + "/teams/" + team.Name() + "/login",
		})
	}

	return methods, nil
}

type byTypeAndName []provider.AuthMethod

func (ms byTypeAndName) Len() int          { return len(ms) }
func (ms byTypeAndName) Swap(i int, j int) { ms[i], ms[j] = ms[j], ms[i] }
func (ms byTypeAndName) Less(i int, j int) bool {
	if ms[i].Type == provider.AuthTypeBasic && ms[j].Type == provider.AuthTypeOAuth {
		return false
	}

	if ms[i].Type == provider.AuthTypeOAuth && ms[j].Type == provider.AuthTypeBasic {
		return true
	}

	return ms[i].DisplayName < ms[j].DisplayName
}
