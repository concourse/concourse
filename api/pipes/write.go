package pipes

import (
	"io"
	"net/http"

	"github.com/concourse/atc"
	"github.com/pivotal-golang/lager"
)

func (s *Server) WritePipe(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("write-pipe")
	pipeID := r.FormValue(":pipe_id")

	dbPipe, err := s.db.GetPipe(pipeID)
	if err != nil {
		logger.Error("failed-to-get-pipe", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if dbPipe.URL == s.url {
		s.pipesL.RLock()
		pipe, found := s.pipes[pipeID]
		s.pipesL.RUnlock()

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		io.Copy(pipe.write, r.Body)
		pipe.write.Close()

		s.pipesL.Lock()
		delete(s.pipes, pipeID)
		s.pipesL.Unlock()
	} else {
		logger.Debug("forwarding-pipe-write-request", lager.Data{"pipe-url": dbPipe.URL})
		response, err := s.forwardRequest(w, r, dbPipe.URL, atc.WritePipe, dbPipe.ID)
		if err != nil {
			logger.Error("failed-to-forward-request", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(response.StatusCode)
	}
}
