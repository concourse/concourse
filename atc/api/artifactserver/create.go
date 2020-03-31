package artifactserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/worker"
)

func (s *Server) CreateArtifact(team db.Team) http.Handler {
	hLog := s.logger.Session("create-artifact")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// TODO: can probably check if fly sent us an etag header
		// which we can lookup in the checksum field
		// that way we don't have to create another volume.

		workerSpec := worker.WorkerSpec{
			TeamID:   team.ID(),
			Platform: r.FormValue("platform"),
		}

		volumeSpec := worker.VolumeSpec{
			Strategy: baggageclaim.EmptyStrategy{},
		}

		volume, err := s.workerClient.CreateVolume(hLog, volumeSpec, workerSpec, db.VolumeTypeArtifact)
		if err != nil {
			hLog.Error("failed-to-create-volume", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// NOTE: there's a race condition here between when the
		// volume gets created and when the artifact gets initialized

		// Within this timeframe there's a chance that the volume could
		// get garbage collected out from under us.

		// This happens because CreateVolume returns a 'created' instead
		// of 'creating' volume.

		// In the long run CreateVolume should probably return a 'creating'
		// volume, but there are other changes needed in FindOrCreateContainer
		// with the way we create volumes for a container

		// I think leaving the race condition is fine for now. Worst case
		// is a fly execute will fail and the user will need to rerun it.

		artifact, err := volume.InitializeArtifact("", 0)
		if err != nil {
			hLog.Error("failed-to-initialize-artifact", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = volume.StreamIn(r.Context(), "/", baggageclaim.GzipEncoding, r.Body)
		if err != nil {
			hLog.Error("failed-to-stream-volume-contents", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)

		json.NewEncoder(w).Encode(present.WorkerArtifact(artifact))
	})
}
