package pipelineserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) RenamePipeline(pipeline db.Pipeline) http.Handler {
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
		warning := atc.ValidateIdentifier(rename.NewName, "pipeline")
		if warning != nil {
			warnings = append(warnings, *warning)
		}

		err = pipeline.Rename(rename.NewName)
		if err != nil {
			logger.Error("failed-to-update-name", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(atc.SaveConfigResponse{Warnings: warnings})
		if err != nil {
			s.logger.Error("failed-to-encode-response", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
