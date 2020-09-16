package teamserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/present"
)

type SetTeamResponse struct {
	Errors   []string            `json:"errors,omitempty"`
	Warnings []atc.ConfigWarning `json:"warnings,omitempty"`
	Team     atc.Team            `json:"team"`
}

func (s *Server) SetTeam(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("set-team")

	hLog.Debug("setting-team")

	acc := accessor.GetAccessor(r)

	teamName := r.FormValue(":team_name")

	var atcTeam atc.Team
	err := json.NewDecoder(r.Body).Decode(&atcTeam)
	if err != nil {
		hLog.Error("malformed-request", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := atcTeam.Validate(); err != nil {
		hLog.Error("malformed-auth-config", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	atcTeam.Name = teamName
	if !acc.IsAdmin() && !acc.IsAuthorized(teamName) {
		hLog.Debug("not-allowed")
		w.WriteHeader(http.StatusForbidden)
		return
	}

	team, found, err := s.teamFactory.FindTeam(teamName)
	if err != nil {
		hLog.Error("failed-to-lookup-team", err, lager.Data{"teamName": teamName})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	response := SetTeamResponse{}
	if found {
		hLog.Debug("updating-credentials")
		err = team.UpdateProviderAuth(atcTeam.Auth)
		if err != nil {
			hLog.Error("failed-to-update-team", err, lager.Data{"teamName": teamName})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	} else if acc.IsAdmin() {
		hLog.Debug("creating team")

		warning := atc.ValidateIdentifier(atcTeam.Name, "team")
		if warning != nil {
			response.Warnings = append(response.Warnings, *warning)
		}

		team, err = s.teamFactory.CreateTeam(atcTeam)
		if err != nil {
			hLog.Error("failed-to-save-team", err, lager.Data{"teamName": teamName})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	err = s.teamFactory.NotifyCacher()
	if err != nil {
		hLog.Error("failed-to-notify-cacher", err, lager.Data{"teamName": teamName})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	response.Team = present.Team(team)

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		hLog.Error("failed-to-encode-team", err)
		w.WriteHeader(http.StatusInternalServerError)
	}

}
