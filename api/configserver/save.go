package configserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/atc"
)

func (s *Server) SaveConfig(w http.ResponseWriter, r *http.Request) {
	var config atc.Config
	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = s.validate(config)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "%s", err)
		return
	}

	err = s.db.SaveConfig(config)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
