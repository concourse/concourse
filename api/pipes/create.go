package pipes

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/nu7hatch/gouuid"
)

func (s *Server) CreatePipe(w http.ResponseWriter, r *http.Request) {
	guid, err := uuid.NewV4()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	pr, pw := io.Pipe()

	pipe := Pipe{
		ID: guid.String(),

		PeerAddr: s.peerAddr,

		read:  pr,
		write: pw,
	}

	s.pipesL.Lock()
	s.pipes[pipe.ID] = pipe
	s.pipesL.Unlock()

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(pipe)
}
