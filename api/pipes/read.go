package pipes

import (
	"io"
	"net/http"
)

func (s *Server) ReadPipe(w http.ResponseWriter, r *http.Request) {
	pipeID := r.FormValue(":pipe_id")

	s.pipesL.RLock()
	pipe, found := s.pipes[pipeID]
	s.pipesL.RUnlock()

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)

	io.Copy(w, pipe.read)

	s.pipesL.Lock()
	delete(s.pipes, pipeID)
	s.pipesL.Unlock()
}
