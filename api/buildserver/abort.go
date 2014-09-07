package buildserver

import (
	"net/http"
	"strconv"

	"github.com/pivotal-golang/lager"
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

	abortURL, err := s.db.AbortBuild(buildID)
	if err != nil {
		aLog.Error("failed-to-set-aborted", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if abortURL != "" {
		resp, err := s.httpClient.Post(abortURL, "", nil)
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
