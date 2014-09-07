package pipes

import (
	"io"
	"net/http"
)

func (s *Server) WritePipe(w http.ResponseWriter, r *http.Request) {
	pipeID := r.FormValue(":pipe_id")

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
}
