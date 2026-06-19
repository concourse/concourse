package buildserver

import (
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) AbortBuild(build db.BuildForAPI) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		aLog := s.logger.Session("abort", build.LagerData())

		err := build.MarkAsAborted()
		if err != nil {
			aLog.Error("failed-to-abort-build", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if _, force := r.URL.Query()[atc.AbortBuildForce]; force {
			if build.Status() == db.BuildStatusPending || build.Status() == db.BuildStatusStarted {
				// Don't wait for containers to be cleaned up. Mark the build as
				// aborted immediately.
				err := build.Finish(db.BuildStatusAborted)
				if err != nil {
					aLog.Error("failed-to-force-abort-build", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
