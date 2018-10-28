package ccserver

import (
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/tedsuo/rata"
)

func (s *Server) GetCC(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("get-cc")
	teamName := rata.Param(r, "team_name")

	logger.Debug("team-not-found", lager.Data{"team": teamName})
	w.WriteHeader(http.StatusNotFound)
}
