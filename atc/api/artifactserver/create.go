package artifactserver

import (
	"fmt"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/worker"
)

func (s *Server) CreateArtifact(team db.Team) http.Handler {
	hLog := s.logger.Session("create-artifact")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		w.Header().Set("Content-Type", "application/json")

		// TODO: can probably check if fly sent us an etag header
		// which we can lookup in the checksum field
		// that way we don't have to create another volume.

		workerSpec := worker.Spec{
			TeamID:   team.ID(),
			Platform: r.FormValue("platform"),
			Tags:     r.Form["tags"],
		}

		volume, artifact, err := s.workerPool.CreateVolumeForArtifact(ctx, workerSpec)
		if err != nil {
			hLog.Error("failed-to-create-volume", err)
			http.Error(w, fmt.Sprintf("%v", err), http.StatusInternalServerError)
			return
		}

		err = volume.StreamIn(r.Context(), "/", compression.NewGzipCompression(), 0, r.Body)
		if err != nil {
			hLog.Error("failed-to-stream-volume-contents", err)
			http.Error(w, fmt.Sprintf("%v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)

		sonic.ConfigDefault.NewEncoder(w).Encode(present.WorkerArtifact(artifact))
	})
}
