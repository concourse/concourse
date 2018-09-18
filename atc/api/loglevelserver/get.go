package loglevelserver

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
)

func (s *Server) GetMinLevel(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("get-min-level")

	var level atc.LogLevel

	switch s.sink.GetMinLevel() {
	case lager.DEBUG:
		level = atc.LogLevelDebug
	case lager.INFO:
		level = atc.LogLevelInfo
	case lager.ERROR:
		level = atc.LogLevelError
	case lager.FATAL:
		level = atc.LogLevelFatal
	default:
		logger.Error("unknown-log-level", nil, lager.Data{
			"level": level,
		})
		level = atc.LogLevelInvalid
	}

	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, "%s", level)
}
