package buildserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/turbine"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
)

func (s *Server) AbortBuild(w http.ResponseWriter, r *http.Request) {
	buildID, err := strconv.Atoi(r.FormValue(":build_id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	aLog := s.logger.Session("abort", lager.Data{
		"build": buildID,
	})

	build, err := s.db.GetBuild(buildID)
	if err != nil {
		aLog.Error("failed-to-get-build", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = s.db.SaveBuildStatus(buildID, db.StatusAborted)
	if err != nil {
		aLog.Error("failed-to-set-aborted", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if build.EngineMetadata != "" {
		var metadata engine.TurbineMetadata
		err := json.Unmarshal([]byte(build.EngineMetadata), &metadata)
		if err != nil {
			aLog.Error("failed-to-unmarshal-metadata", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		generator := rata.NewRequestGenerator(metadata.Endpoint, turbine.Routes)

		abort, err := generator.CreateRequest(
			turbine.AbortBuild,
			rata.Params{"guid": metadata.Guid},
			nil,
		)
		if err != nil {
			aLog.Error("failed-to-construct-abort-request", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp, err := s.httpClient.Do(abort)
		if err != nil {
			aLog.Error("failed-to-abort-build", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp.Body.Close()

		w.WriteHeader(resp.StatusCode)
		return
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}
