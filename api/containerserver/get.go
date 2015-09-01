package containerserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc/api/present"
	"github.com/pivotal-golang/lager"
)

func (s *Server) GetContainer(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":id")

	hLog := s.logger.Session("container", lager.Data{
		"handle": handle,
	})

	container, err := s.workerClient.LookupContainer(handle)
	if err != nil {
		hLog.Info("failed-to-get-container", lager.Data{"error": err})
		if _, ok := err.(garden.ContainerNotFoundError); ok {
			http.Error(w, fmt.Sprintf("failed to get container: %s", err), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("failed to get container: %s", err), http.StatusInternalServerError)
		}
		return
	}

	presentedContainer := present.Container(container)
	container.Release()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(presentedContainer)
}
