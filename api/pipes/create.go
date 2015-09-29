package pipes

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/nu7hatch/gouuid"

	"github.com/concourse/atc"
)

func (s *Server) CreatePipe(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("create-pipe")
	guid, err := uuid.NewV4()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = s.db.CreatePipe(guid.String(), s.url)
	if err != nil {
		logger.Error("failed-to-create-pipe", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	pr, pw := io.Pipe()

	pipeResource := atc.Pipe{
		ID: guid.String(),
	}

	pipe := pipe{
		resource: pipeResource,

		read:  pr,
		write: pw,
	}

	s.pipesL.Lock()
	s.pipes[pipeResource.ID] = pipe
	s.pipesL.Unlock()

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(pipeResource)
}
