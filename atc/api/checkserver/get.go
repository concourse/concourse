package checkserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/present"
)

func (s *Server) GetCheck(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("get-check")

	checkID, err := strconv.Atoi(r.FormValue(":check_id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	check, found, err := s.checkFactory.Check(checkID)
	if err != nil {
		logger.Error("could-not-get-check", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	checkables, err := check.AllCheckables()
	if err != nil {
		logger.Error("failed-to-get-checkables", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	acc := accessor.GetAccessor(r)

	for _, checkable := range checkables {

		if acc.IsAuthorized(checkable.TeamName()) {

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			err = json.NewEncoder(w).Encode(present.Check(check))
			if err != nil {
				logger.Error("failed-to-encode-check", err)
				w.WriteHeader(http.StatusInternalServerError)
			}

			return
		}
	}

	w.WriteHeader(http.StatusForbidden)
}
