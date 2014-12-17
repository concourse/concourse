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

	closed := w.(http.CloseNotifier).CloseNotify()

	w.WriteHeader(http.StatusOK)

	w.(http.Flusher).Flush()

	copied := make(chan struct{})
	go func() {
		io.Copy(w, pipe.read)
		close(copied)
	}()

dance:
	for {
		select {
		case <-copied:
			break dance
		case <-closed:
			// connection died; terminate the pipe
			pipe.write.Close()
		}
	}

	s.pipesL.Lock()
	delete(s.pipes, pipeID)
	s.pipesL.Unlock()
}
