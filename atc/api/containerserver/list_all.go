package containerserver

import (
	"encoding/json"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
	"net/http"
	"time"
)

// show all public containers and team private containers if authorized
func (s *Server) ListAllContainers(w http.ResponseWriter, r *http.Request) {

	logger := s.logger.Session("list-all-containers")

	acc := accessor.GetAccessor(r)

	var containers []db.Container
	var err error

	if acc.IsAdmin() {
		containers, err = s.containerRepository.AllContainers()
	} else {
		containers, err = s.containerRepository.VisibleContainers(acc.TeamNames())
	}

	if err != nil {
		logger.Error("failed-to-get-all-visible-containers", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	presentedContainers := []atc.Container{}
	for _, container := range containers {
		presentedContainers = append(presentedContainers, present.Container(container, time.Time{}))
	}
	err = json.NewEncoder(w).Encode(presentedContainers)
	if err != nil {
		logger.Error("failed-to-encode-containers", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
