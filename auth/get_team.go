package auth

import "net/http"

type Team interface {
	Name() string
	IsAdmin() bool
	IsAuthorized(teamName string) bool
}

type team struct {
	name    string
	isAdmin bool
}

func (t *team) Name() string {
	return t.name
}

func (t *team) IsAdmin() bool {
	return t.isAdmin
}

func (t *team) IsAuthorized(teamName string) bool {
	return t.name == teamName
}

func GetTeam(r *http.Request) (Team, bool) {
	teamName, namePresent := r.Context().Value(teamNameKey).(string)
	isAdmin, adminPresent := r.Context().Value(isAdminKey).(bool)

	if !(namePresent && adminPresent) {
		return nil, false
	}

	return &team{
		name:    teamName,
		isAdmin: isAdmin,
	}, true
}
