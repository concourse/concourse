package usersserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
)

func (s *Server) GetUser(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("user")
	w.Header().Set("Content-Type", "application/json")

	info := accessor.GetAccessor(r).UserInfo()

	user := atc.UserInfo{
		Sub:      info.Sub,
		Name:     info.Name,
		UserId:   info.UserID,
		UserName: info.UserName,
		Email:    info.Email,
		IsAdmin:  info.IsAdmin,
		IsSystem: info.IsSystem,
		Teams:    info.Teams,
	}

	err := json.NewEncoder(w).Encode(user)
	if err != nil {
		hLog.Error("failed-to-encode-users", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	return
}
