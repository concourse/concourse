package containerserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/api/present"
	"github.com/pivotal-golang/lager"
)

func (s *Server) GetContainer(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":id")

	hLog := s.logger.Session("container", lager.Data{
		"handle": handle,
	})

	container, found, err := s.db.GetContainerInfo(handle)
	if err != nil {
		hLog.Error("Failed to lookup container", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !found {
		hLog.Info("Failed to find container")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	presentedContainer := present.Container(container)

	hLog.Info("Found container", lager.Data{"container": presentedContainer})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(presentedContainer)
}
