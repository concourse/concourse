package containerserver

import (
	"encoding/json"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) GetContainer(team db.Team) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handle := r.FormValue(":id")

		hLog := s.logger.Session("container", lager.Data{
			"handle": handle,
		})

		container, found, err := team.FindContainerByHandle(handle)
		if err != nil {
			hLog.Error("failed-to-lookup-container", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			hLog.Debug("container-not-found")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		isCheckContainer, err := team.IsCheckContainer(handle)
		if err != nil {
			hLog.Error("failed-to-find-container", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		ok, err := team.IsContainerWithinTeam(handle, isCheckContainer)
		if err != nil {
			hLog.Error("failed-to-find-container-within-team", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !ok {
			hLog.Error("container-not-found-within-team", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		hLog.Debug("found-container")

		presentedContainer := present.Container(container, time.Time{})

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(presentedContainer)
		if err != nil {
			hLog.Error("failed-to-encode-container", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
