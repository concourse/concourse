package authserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web"
	"github.com/google/jsonapi"
	"github.com/tedsuo/rata"
)

const BasicAuthDisplayName = "Basic Auth"

func (s *Server) ListAuthMethods(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-auth-methods")

	w.Header().Set("Content-Type", "application/json")

	teamName := r.FormValue(":team_name")
	team, found, err := s.teamFactory.FindTeam(teamName)
	if err != nil {
		logger.Error("failed-to-get-team", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !found {
		logger.Info("team-not-found")
		w.WriteHeader(http.StatusNotFound)
		jsonapi.MarshalErrors(w, []*jsonapi.ErrorObject{{
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
	json.NewEncoder(w).Encode(methods)
}

type byTypeAndName []atc.AuthMethod

func (ms byTypeAndName) Len() int          { return len(ms) }
func (ms byTypeAndName) Swap(i int, j int) { ms[i], ms[j] = ms[j], ms[i] }
func (ms byTypeAndName) Less(i int, j int) bool {
	if ms[i].Type == atc.AuthTypeBasic && ms[j].Type == atc.AuthTypeOAuth {
		return false
	}

	if ms[i].Type == atc.AuthTypeOAuth && ms[j].Type == atc.AuthTypeBasic {
		return true
	}

	return ms[i].DisplayName < ms[j].DisplayName
}

func (s *Server) authMethods(team db.Team) ([]atc.AuthMethod, error) {
	methods := []atc.AuthMethod{}

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
		path, err := web.Routes.CreatePathForRoute(
			web.TeamLogIn,
			rata.Params{"team_name": team.Name()},
		)
		if err != nil {
			return nil, err
		}

		methods = append(methods, atc.AuthMethod{
			Type:        atc.AuthTypeBasic,
			DisplayName: BasicAuthDisplayName,
			AuthURL:     s.externalURL + path,
		})
	}

	return methods, nil
}
