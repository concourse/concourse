package resourceserver

import (
	"net/http"

	"github.com/tedsuo/rata"
)

func (s *Server) UnpauseResource(w http.ResponseWriter, r *http.Request) {
	resourceName := rata.Param(r, "resource_name")

	err := s.resourceDB.UnpauseResource(resourceName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
