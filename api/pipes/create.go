package pipes

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/nu7hatch/gouuid"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
)

func (s *Server) CreatePipe(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("create-pipe")
	guid, err := uuid.NewV4()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	authTeam, found := auth.GetTeam(r)
	if !found {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = s.db.CreatePipe(guid.String(), s.url, authTeam.Name())
	if err != nil {
		logger.Error("failed-to-create-pipe", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	pr, pw := io.Pipe()

	pipeID := guid.String()

	reqGen := rata.NewRequestGenerator(s.externalURL, atc.Routes)

	readReq, err := reqGen.CreateRequest(atc.ReadPipe, rata.Params{
		"pipe_id": pipeID,
	}, nil)
	if err != nil {
		logger.Error("failed-to-create-pipe", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	writeReq, err := reqGen.CreateRequest(atc.WritePipe, rata.Params{
		"pipe_id": pipeID,
	}, nil)
	if err != nil {
		logger.Error("failed-to-create-pipe", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	pipeResource := atc.Pipe{
		ID:       pipeID,
		ReadURL:  readReq.URL.String(),
		WriteURL: writeReq.URL.String(),
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
