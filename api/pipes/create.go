package pipes

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/nu7hatch/gouuid"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/auth"
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

	team, found, err := s.teamFactory.FindTeam(authTeam.Name())
	if err != nil {
		logger.Error("failed-to-find-team", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		logger.Debug("team-not-found")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = team.CreatePipe(guid.String(), s.url)
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

	err = json.NewEncoder(w).Encode(pipeResource)
	if err != nil {
		logger.Error("failed-to-encode-pipe", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
