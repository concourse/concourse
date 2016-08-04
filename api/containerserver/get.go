package containerserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
	"code.cloudfoundry.org/lager"
)

func (s *Server) GetContainer(w http.ResponseWriter, r *http.Request) {
	teamName := auth.GetAuthOrDefaultTeamName(r)
	handle := r.FormValue(":id")

	hLog := s.logger.Session("container", lager.Data{
		"handle": handle,
	})

	teamDB := s.teamDBFactory.GetTeamDB(teamName)
	container, found, err := teamDB.GetContainer(handle)
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

	hLog.Debug("found-container")

	presentedContainer := present.Container(container)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(presentedContainer)
}
