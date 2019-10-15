package pipelineserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/db"
)

// MovePipeline allows authorized user to move pipeline from one team to another
func (s *Server) MovePipeline(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("move-pipeline")

		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Error("failed-to-read-body", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var move atc.MoveRequest
		err = json.Unmarshal(data, &move)
		if err != nil {
			logger.Error("failed-to-unmarshal-body", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		acc := accessor.GetAccessor(r)
		if acc.IsAuthorized(move.DestinationTeamName) {
			err = pipeline.Move(move.DestinationTeamName)

			if err != nil {
				logger.Error("failed-to-move-pipeline", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			logger.Error("user-not-authorized-to-move-pipeline", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
