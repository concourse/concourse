package pipelineserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) MovePipeline(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("move-pipeline")

		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Error("failed-to-read-body", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var rename atc.MoveRequest
		err = json.Unmarshal(data, &move)
		if err != nil {
			logger.Error("failed-to-unmarshal-body", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = pipeline.Rename(move.newTeamName)
		if err != nil {
			logger.Error("failed-to-move-pipeline", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
