package componentsserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/present"
)

func (s *Server) GetComponents(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("get-components")
	comps, err := s.componentFactory.All()
	if err != nil {
		logger.Error("failed-to-get-all", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	atcComponents := make([]atc.Component, len(comps))
	for i, c := range comps {
		atcComponents[i] = present.Component(c)
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(atcComponents)
	if err != nil {
		logger.Error("failed-to-encode-components", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Server) PauseAll(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("pause-all-components")
	err := s.componentFactory.PauseAll()
	if err != nil {
		logger.Error("error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Server) UnpauseAll(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("unpause-all-components")
	err := s.componentFactory.UnpauseAll()
	if err != nil {
		logger.Error("error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
