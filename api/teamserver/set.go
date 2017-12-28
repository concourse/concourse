package teamserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/auth"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
	"github.com/concourse/skymarshal/provider"
)

func (s *Server) SetTeam(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("set-team")

	hLog.Debug("setting-team")

	authTeam, authTeamFound := auth.GetTeam(r)
	if !authTeamFound {
		hLog.Error("failed-to-get-team-from-auth", errors.New("failed-to-get-team-from-auth"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	teamName := r.FormValue(":team_name")

	var atcTeam atc.Team
	err := json.NewDecoder(r.Body).Decode(&atcTeam)
	if err != nil {
		hLog.Error("malformed-request", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	atcTeam.Name = teamName
	if !authTeam.IsAdmin() && !authTeam.IsAuthorized(teamName) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	hLog.Debug("configured-authentication", lager.Data{"BasicAuth": atcTeam.BasicAuth, "ProviderAuth": atcTeam.Auth})

	if atcTeam.BasicAuth != nil {
		if atcTeam.BasicAuth.BasicAuthUsername == "" || atcTeam.BasicAuth.BasicAuthPassword == "" {
			hLog.Info("missing-basic-auth-username-or-password")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	providers := provider.GetProviders()

	for providerName, config := range atcTeam.Auth {
		p, found := providers[providerName]
		if !found {
			hLog.Error("failed-to-find-provider", err, lager.Data{"provider": providerName})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		authConfig, err := p.UnmarshalConfig(config)
		if err != nil {
			hLog.Error("failed-to-unmarshall-auth", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = authConfig.Validate()
		if err != nil {
			hLog.Error("request-body-validation-error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	team, found, err := s.teamFactory.FindTeam(teamName)
	if err != nil {
		hLog.Error("failed-to-lookup-team", err, lager.Data{"teamName": teamName})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if found {
		hLog.Debug("updating-credentials")
		err = s.updateCredentials(atcTeam, team)
		if err != nil {
			hLog.Error("failed-to-update-team", err, lager.Data{"teamName": teamName})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	} else if authTeam.IsAdmin() {
		hLog.Debug("creating team")

		team, err = s.teamFactory.CreateTeam(atcTeam)
		if err != nil {
			hLog.Error("failed-to-save-team", err, lager.Data{"teamName": teamName})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	err = json.NewEncoder(w).Encode(present.Team(team))
	if err != nil {
		hLog.Error("failed-to-encode-team", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Server) updateCredentials(atcTeam atc.Team, team db.Team) error {
	err := team.UpdateBasicAuth(atcTeam.BasicAuth)
	if err != nil {
		return err
	}

	err = team.UpdateProviderAuth(atcTeam.Auth)
	return err
}
