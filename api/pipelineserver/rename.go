package pipelineserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

func (s *Server) RenamePipeline(pipelineDB db.PipelineDB, _ dbng.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			s.logger.Error("call-to-update-pipeline-name-copy-failed", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var value struct{ Name string }
		err = json.Unmarshal(data, &value)
		if err != nil {
			s.logger.Error("call-to-update-pipeline-name-unmarshal-failed", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = pipelineDB.UpdateName(value.Name)
		if err != nil {
			s.logger.Error("call-to-update-pipeline-name-failed", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
