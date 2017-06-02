package buildserver

import (
	"net/http"

	"github.com/concourse/atc/db"

	"code.cloudfoundry.org/lager"
)

func (s *Server) AbortBuild(build db.Build) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		aLog := s.logger.Session("abort", lager.Data{
			"build": build.ID(),
		})

		engineBuild, err := s.engine.LookupBuild(aLog, build)
		if err != nil {
			aLog.Error("failed-to-lookup-build", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = engineBuild.Abort(aLog)
		if err != nil {
			aLog.Error("failed-to-abort-build", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
