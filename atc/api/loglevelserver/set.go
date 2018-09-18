package loglevelserver

import (
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
)

func (s *Server) SetMinLevel(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
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
