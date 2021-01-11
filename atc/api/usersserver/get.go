package usersserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc/api/accessor"
)

func (s *Server) GetUser(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("user")
	w.Header().Set("Content-Type", "application/json")

	acc := accessor.GetAccessor(r)

	err := json.NewEncoder(w).Encode(acc.UserInfo())
	if err != nil {
		hLog.Error("failed-to-encode-users", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	return
}
