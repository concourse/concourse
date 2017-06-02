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

		volumes, err := s.factory.GetTeamVolumes(team.ID())
		if err != nil {
			hLog.Error("failed-to-find-volumes", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		hLog.Debug("listed", lager.Data{"volume-count": len(volumes)})

		presentedVolumes := make([]atc.Volume, len(volumes))
		for i := 0; i < len(volumes); i++ {
			volume := volumes[i]
			presentedVolumes[i], err = present.Volume(volume)
			if err != nil {
				hLog.Error("failed-to-present-volume", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		json.NewEncoder(w).Encode(presentedVolumes)
	})
}
