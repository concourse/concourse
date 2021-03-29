package pipelineserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) OrderPipelinesWithinGroup(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("order-pipelines-within-group")

	var instanceVars []atc.InstanceVars
	if err := json.NewDecoder(r.Body).Decode(&instanceVars); err != nil {
		logger.Error("invalid-json", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

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
		return
	}

	groupName := r.FormValue(":pipeline_name")

	err = team.OrderPipelinesWithinGroup(groupName, instanceVars)
	if err != nil {
		logger.Error("failed-to-order-pipelines", err, lager.Data{
			"pipeline_name": groupName,
			"instance_vars": instanceVars,
		})
		var errNotFound db.ErrPipelineNotFound
		if errors.As(err, &errNotFound) {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, err.Error())
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}
