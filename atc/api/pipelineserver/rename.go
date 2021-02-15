package pipelineserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) RenamePipeline(team db.Team) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("rename-pipeline")

		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Error("failed-to-read-body", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var rename atc.RenameRequest
		err = json.Unmarshal(data, &rename)
		if err != nil {
			logger.Error("failed-to-unmarshal-body", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var warnings []atc.ConfigWarning
		var errs []string
		warning, err := atc.ValidateIdentifier(rename.NewName, "pipeline")
		if err != nil {
			errs = append(errs, err.Error())
		}
		if warning != nil {
			warnings = append(warnings, *warning)
		}

		oldName := r.FormValue(":pipeline_name")
		found, err := team.RenamePipeline(oldName, rename.NewName)
		if err != nil {
			logger.Error("failed-to-update-name", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !found {
			logger.Info("pipeline-not-found", lager.Data{"pipeline_name": oldName})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(atc.SaveConfigResponse{Warnings: warnings, Errors: errs})
		if err != nil {
			logger.Error("failed-to-encode-response", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
