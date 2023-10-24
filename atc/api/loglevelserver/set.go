package loglevelserver

import (
	"io"
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc"
)

func (s *Server) SetMinLevel(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var level lager.LogLevel

	switch atc.LogLevel(string(body)) {
	case atc.LogLevelDebug:
		level = lager.DEBUG
	case atc.LogLevelInfo:
		level = lager.INFO
	case atc.LogLevelError:
		level = lager.ERROR
	case atc.LogLevelFatal:
		level = lager.FATAL
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s.sink.SetMinLevel(level)
}
