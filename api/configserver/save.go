package configserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/atc"
	"github.com/pivotal-golang/lager"
)

func (s *Server) SaveConfig(w http.ResponseWriter, r *http.Request) {
	session := s.logger.Session("set-config")

	var config atc.Config
	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		session.Error("malformed-json", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = s.validate(config)
	if err != nil {
		session.Error("ignoring-invalid-config", err)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "%s", err)
		return
	}

	session.Info("saving", lager.Data{"config": config})

	err = s.db.SaveConfig(config)
	if err != nil {
		session.Error("failed-to-save-config", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	session.Info("saved")

	w.WriteHeader(http.StatusOK)
}
