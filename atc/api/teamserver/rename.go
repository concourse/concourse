package teamserver

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/concourse/concourse/atc"
	. "github.com/concourse/concourse/atc/api/helpers"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) RenameTeam(team db.Team) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("rename-team")

		data, err := io.ReadAll(r.Body)
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

		var configErrors []atc.ConfigErrors
		var errs []string
		configError := atc.ValidateIdentifier(rename.NewName, "team")
		if configError != nil {
			errs = append(errs, configError.Error())
			HandleBadRequest(w, errs...)
			return
		}

		err = team.Rename(rename.NewName)
		if err != nil {
			logger.Error("failed-to-update-team-name", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(atc.SaveConfigResponse{ConfigErrors: configErrors, Errors: errs})
		if err != nil {
			s.logger.Error("failed-to-encode-response", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
