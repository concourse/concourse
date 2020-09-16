package artifactserver

import (
	"io"
	"net/http"
	"strconv"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) GetArtifact(team db.Team) http.Handler {
	logger := s.logger.Session("get-artifact")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Content-Type", "application/octet-stream")

		artifactID, err := strconv.Atoi(r.FormValue(":artifact_id"))
		if err != nil {
			logger.Error("failed-to-get-artifact-id", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		artifactVolume, found, err := team.FindVolumeForWorkerArtifact(artifactID)
		if err != nil {
			logger.Error("failed-to-get-artifact-volume", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Error("failed-to-find-artifact-volume", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		workerVolume, found, err := s.workerClient.FindVolume(logger, team.ID(), artifactVolume.Handle())
		if err != nil {
			logger.Error("failed-to-get-worker-volume", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Error("failed-to-find-worker-volume", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		reader, err := workerVolume.StreamOut(r.Context(), "/", baggageclaim.GzipEncoding)
		if err != nil {
			logger.Error("failed-to-stream-volume-contents", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		defer reader.Close()

		_, err = io.Copy(w, reader)
		if err != nil {
			logger.Error("failed-to-encode-artifact", err)
		}
	})
}
