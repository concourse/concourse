package componentsserver

import (
	"net/http"

	"code.cloudfoundry.org/lager/v3"
)

func (s *Server) Pause(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get(":component_name")
	logger := s.logger.Session("pause-component", lager.Data{"name": name})

	c, found, err := s.componentFactory.Find(name)
	if err != nil {
		logger.Error("failed-to-find", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = c.Pause()
	if err != nil {
		logger.Error("failed-to-pause", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Server) Unpause(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get(":component_name")
	logger := s.logger.Session("unpause-component", lager.Data{"name": name})

	c, found, err := s.componentFactory.Find(name)
	if err != nil {
		logger.Error("failed-to-find", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = c.Unpause()
	if err != nil {
		logger.Error("failed-to-unpause", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
