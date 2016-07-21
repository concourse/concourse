package teamserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
)

func (s *Server) SetTeam(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("create-team")
	hLog.Debug("setting team")

	requesterTeamName, _, isAdmin, found := auth.GetTeam(r)

	if !found {
		hLog.Error("failed-to-get-team-from-auth", errors.New("failed-to-get-team-from-auth"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	teamName := r.FormValue(":team_name")

	teamDB := s.teamDBFactory.GetTeamDB(teamName)

	var team db.Team
	err := json.NewDecoder(r.Body).Decode(&team)
	if err != nil {
		hLog.Error("malformed-request", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	team.Name = teamName
	if !isAdmin {
		if teamName != requesterTeamName {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	err = s.validate(team)
	if err != nil {
		hLog.Error("request-body-validation-error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	savedTeam, found, err := teamDB.GetTeam()
	if err != nil {
		hLog.Error("failed-to-get-team", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if found {
		hLog.Debug("updating credentials")
		err = s.updateCredentials(team, teamDB)
		if err != nil {
			hLog.Error("failed-to-update-team", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	} else if isAdmin {
		hLog.Debug("creating team")

		savedTeam, err = s.teamsDB.CreateTeam(team)
		if err != nil {
			hLog.Error("failed-to-save-team", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	json.NewEncoder(w).Encode(present.Team(savedTeam))
}

func (s *Server) updateCredentials(team db.Team, teamDB db.TeamDB) error {
	_, err := teamDB.UpdateBasicAuth(team.BasicAuth)
	if err != nil {
		return err
	}

	_, err = teamDB.UpdateGitHubAuth(team.GitHubAuth)
	if err != nil {
		return err
	}

	_, err = teamDB.UpdateUAAAuth(team.UAAAuth)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) validate(team db.Team) error {
	if team.BasicAuth != nil {
		if team.BasicAuth.BasicAuthUsername == "" || team.BasicAuth.BasicAuthPassword == "" {
			return errors.New("basic auth missing BasicAuthUsername or BasicAuthPassword")
		}
	}

	if team.GitHubAuth != nil {
		if team.GitHubAuth.ClientID == "" || team.GitHubAuth.ClientSecret == "" {
			return errors.New("GitHub auth missing ClientID or ClientSecret")
		}

		if len(team.GitHubAuth.Organizations) == 0 &&
			len(team.GitHubAuth.Teams) == 0 &&
			len(team.GitHubAuth.Users) == 0 {
			return errors.New("GitHub auth requires at least one Organization, Team, or User")
		}
	}

	if team.UAAAuth != nil {
		if team.UAAAuth.ClientID == "" || team.UAAAuth.ClientSecret == "" {
			return errors.New("CF auth missing ClientID or ClientSecret")
		}

		if len(team.UAAAuth.CFSpaces) == 0 {
			return errors.New("CF auth requires at least one Space")
		}

		if team.UAAAuth.AuthURL == "" || team.UAAAuth.TokenURL == "" || team.UAAAuth.CFURL == "" {
			return errors.New("CF auth requires AuthURL, TokenURL and APIURL")
		}
	}

	return nil
}
