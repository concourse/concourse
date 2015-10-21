package volumeserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/pivotal-golang/lager"
)

func (s *Server) ListVolumes(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("list-volumes")

	hLog.Debug("listing")

	volumes, err := s.db.GetVolumes()
	if err != nil {
		hLog.Error("failed-to-find-volumes", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	hLog.Debug("listed", lager.Data{"volume-count": len(volumes)})

	presentedVolumes := make([]atc.Volume, len(volumes))
	for i := 0; i < len(volumes); i++ {
		volume := volumes[i]
		presentedVolumes[i] = present.Volume(volume.VolumeData)
	}

	json.NewEncoder(w).Encode(presentedVolumes)
}
