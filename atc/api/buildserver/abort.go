package buildserver

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) AbortBuild(build db.Build) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		aLog := s.logger.Session("abort", lager.Data{
			"build": build.ID(),
		})

		err := build.MarkAsAborted()
		if err != nil {
			aLog.Error("failed-to-abort-build", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
