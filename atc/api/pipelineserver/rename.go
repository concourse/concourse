package pipelineserver

import (
	"encoding/json"
	"errors"
	"fmt"
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
		if err = atc.ValidateIdentifier(rename.NewName, "pipeline"); err != nil {
			var got *atc.InvalidIdentifierError
			if errors.As(err, &got) {
				warnings = append(warnings, got.ConfigWarning())
			}
		}

		err = pipeline.Rename(rename.NewName)
		if err != nil {
			logger.Error("failed-to-update-name", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		s.writeResponse(w, atc.SaveConfigResponse{Warnings: warnings})
	})
}

func (s *Server) writeResponse(w http.ResponseWriter, response atc.SaveConfigResponse) {
	responseJSON, err := json.Marshal(response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to generate error response: %s", err)
		return
	}

	_, err = w.Write(responseJSON)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
