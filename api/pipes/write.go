package pipes

import (
	"errors"
	"io"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/api/accessor"
)

func (s *Server) WritePipe(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("write-pipe")
	pipeID := r.FormValue(":pipe_id")
	teamName := r.FormValue(":team_name")

	acc := accessor.GetAccessor(r)

	if !acc.IsAuthorized(teamName) {
		logger.Error("team-not-authorized-to-read-pipe",
			errors.New("team-not-authorized-to-read-pipe"),
			lager.Data{"TeamName": teamName, "PipeID": pipeID})
		w.WriteHeader(http.StatusForbidden)
		return
	}

	dbTeam, found, err := s.teamFactory.FindTeam(teamName)
	if err != nil {
		logger.Error("failed-to-get-team-from-db", errors.New("failed-to-get-team-from-db"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !found {
		logger.Error("failed-to-find-team", errors.New("failed-to-find-team"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	dbPipe, err := dbTeam.GetPipe(pipeID)
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

		_, _ = io.Copy(pipe.write, r.Body)
		_ = pipe.write.Close()

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
