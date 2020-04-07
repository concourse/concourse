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

	acc := accessor.GetAccessor(r)

	claims := acc.Claims()

	user := atc.UserInfo{
		Sub:      claims.Sub,
		Name:     claims.Name,
		UserId:   claims.UserID,
		UserName: claims.UserName,
		Email:    claims.Email,
		IsAdmin:  acc.IsAdmin(),
		IsSystem: acc.IsSystem(),
		Teams:    acc.TeamRoles(),
	}

	err := json.NewEncoder(w).Encode(user)
	if err != nil {
		hLog.Error("failed-to-encode-users", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	return
}
