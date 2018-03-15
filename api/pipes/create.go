package pipes

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/nu7hatch/gouuid"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/accessor"
)

func (s *Server) CreatePipe(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("create-pipe")

	teamName := r.FormValue(":team_name")
	acc := accessor.GetAccessor(r)

	if !acc.IsAuthorized(teamName) {
		logger.Error("team-not-authorized-to-create-pipe",
			errors.New("team-not-authorized-to-create-pipe"),
			lager.Data{"TeamName": teamName})
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	guid, err := uuid.NewV4()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	team, found, err := s.teamFactory.FindTeam(teamName)
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
		"team_name": teamName,
		"pipe_id":   pipeID,
	}, nil)
	if err != nil {
		logger.Error("failed-to-create-pipe", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	writeReq, err := reqGen.CreateRequest(atc.WritePipe, rata.Params{
		"team_name": teamName,
		"pipe_id":   pipeID,
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
