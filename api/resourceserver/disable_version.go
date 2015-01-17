package resourceserver

import (
	"net/http"
	"strconv"
)

func (s *Server) DisableResourceVersion(w http.ResponseWriter, r *http.Request) {
	resourceID, err := strconv.Atoi(r.FormValue(":version_id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = s.resourceDB.DisableVersionedResource(resourceID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
