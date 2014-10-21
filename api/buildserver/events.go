package buildserver

import (
	"net/http"
	"strconv"
)

func (s *Server) BuildEvents(w http.ResponseWriter, r *http.Request) {
	buildID, err := strconv.Atoi(r.FormValue(":build_id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	streamDone := make(chan struct{})

	go func() {
		defer close(streamDone)
		s.eventHandlerFactory(s.db, buildID, nil).ServeHTTP(w, r)
	}()

	select {
	case <-streamDone:
	case <-s.drain:
	}
}
