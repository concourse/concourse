package authserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/web"
	"github.com/tedsuo/rata"
)

const BasicAuthDisplayName = "Basic Auth"

func (s *Server) ListAuthMethods(w http.ResponseWriter, r *http.Request) {
	teamName := r.FormValue(":team_name")
	teamDB := s.teamDBFactory.GetTeamDB(teamName)
	team, found, err := teamDB.GetTeam()
	if err != nil {
		s.logger.Error("failed-to-get-team", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !found {
		s.logger.Info("team-not-found")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	methods := []atc.AuthMethod{}
	providers, err := s.providerFactory.GetProviders(teamName)
	if err != nil {
		s.logger.Error("failed-to-get-providers", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	for name, provider := range providers {
		path, err := auth.OAuthRoutes.CreatePathForRoute(
			auth.OAuthBegin,
			rata.Params{"provider": name},
		)
		if err != nil {
			s.logger.Error("failed-to-create-path-for-route", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		path = path + fmt.Sprintf("?team_name=%s", teamName)
		methods = append(methods, atc.AuthMethod{
			Type:        atc.AuthTypeOAuth,
			DisplayName: provider.DisplayName(),
			AuthURL:     s.oAuthBaseURL + path,
		})
	}

	if team.BasicAuth != nil {
		path, err := web.Routes.CreatePathForRoute(
			web.TeamLogIn,
			rata.Params{"team_name": teamName},
		)
		if err != nil {
			s.logger.Error("failed-to-create-path-for-route", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		methods = append(methods, atc.AuthMethod{
			Type:        atc.AuthTypeBasic,
			DisplayName: BasicAuthDisplayName,
			AuthURL:     s.externalURL + path,
		})
	}

	sort.Sort(byTypeAndName(methods))

	w.Header().Set("Content-Type", "application/json")
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
