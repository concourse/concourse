package pipes

import (
	"errors"
	"io"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/api/auth"
)

func (s *Server) ReadPipe(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("read-pipe")
	pipeID := r.FormValue(":pipe_id")

	authTeam, found := auth.GetTeam(r)
	if !found {
		logger.Error("failed-to-get-team", errors.New("failed-to-get-team"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	dbTeam, found, err := s.teamFactory.FindTeam(authTeam.Name())
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

	if authTeam.Name() != dbPipe.TeamName {
		logger.Error("team-not-authorized-to-read-pipe",
			errors.New("team-not-authorized-to-read-pipe"),
			lager.Data{"TeamName": authTeam.Name(), "PipeID": dbPipe.ID})
		w.WriteHeader(http.StatusForbidden)
		return
	}

	closed := w.(http.CloseNotifier).CloseNotify()

	if dbPipe.URL == s.url {
		s.pipesL.RLock()
		pipe, found := s.pipes[pipeID]
		s.pipesL.RUnlock()

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusOK)

		w.(http.Flusher).Flush()

		copied := make(chan struct{})
		go func() {
			_, _ = io.Copy(w, pipe.read)
			close(copied)
		}()

	dance:
		for {
			select {
			case <-copied:
				break dance
			case <-closed:
				// connection died; terminate the pipe
				_ = pipe.write.Close()
			}
		}

		s.pipesL.Lock()
		delete(s.pipes, pipeID)
		s.pipesL.Unlock()
	} else {
		logger.Debug("forwarding-pipe-read-request", lager.Data{"pipe-url": dbPipe.URL})
		response, err := s.forwardRequest(w, r, dbPipe.URL, atc.ReadPipe, dbPipe.ID)
		if err != nil {
			logger.Error("failed-to-forward-request", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(response.StatusCode)

		w.(http.Flusher).Flush()

		copied := make(chan struct{})
		go func() {
			_, _ = io.Copy(w, response.Body)
			close(copied)
		}()

	danceMore:
		for {
			select {
			case <-copied:
				break danceMore
			case <-closed:
				// connection died; terminate the pipe
				w.WriteHeader(http.StatusGatewayTimeout)
			}
		}
	}
}
