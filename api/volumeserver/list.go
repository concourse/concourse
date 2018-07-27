package volumeserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) ListVolumes(team db.Team) http.Handler {
	hLog := s.logger.Session("list-volumes")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hLog.Debug("listing")

		volumes, err := s.repository.GetTeamVolumes(team.ID())
		if err != nil {
			hLog.Error("failed-to-find-volumes", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		hLog.Debug("listed", lager.Data{"volume-count": len(volumes)})

		presentedVolumes := []atc.Volume{}
		for i := 0; i < len(volumes); i++ {
			volume := volumes[i]
			if vol, err := present.Volume(volume); err != nil {
				hLog.Error("failed-to-present-volume", err)
			} else {
				presentedVolumes = append(presentedVolumes, vol)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(presentedVolumes)
		if err != nil {
			hLog.Error("failed-to-encode-volumes", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
