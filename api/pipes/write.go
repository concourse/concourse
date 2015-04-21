package pipes

import (
	"io"
	"net/http"

	"github.com/concourse/atc"
)

func (s *Server) WritePipe(w http.ResponseWriter, r *http.Request) {
	pipeID := r.FormValue(":pipe_id")

	dbPipe, err := s.db.GetPipe(pipeID)
	if err != nil {
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

		response, err := s.forwardRequest(w, r, dbPipe.URL, atc.WritePipe, dbPipe.ID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(response.StatusCode)
	}
}
