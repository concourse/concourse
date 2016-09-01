package auth

import "net/http"

type Team interface {
	Name() string
	ID() int
	IsAdmin() bool
	IsAuthorized(teamName string) bool
}

type team struct {
	name    string
	teamID  int
	isAdmin bool
}

func (t *team) Name() string {
	return t.name
}

func (t *team) ID() int {
	return t.teamID
}

func (t *team) IsAdmin() bool {
	return t.isAdmin
}

func (t *team) IsAuthorized(teamName string) bool {
	return t.name == teamName
}

func GetTeam(r *http.Request) (Team, bool) {
	teamName, namePresent := r.Context().Value(teamNameKey).(string)
	teamID, teamIDPresent := r.Context().Value(teamIDKey).(int)
	isAdmin, adminPresent := r.Context().Value(isAdminKey).(bool)

	if !(namePresent && teamIDPresent && adminPresent) {
		return nil, false
	}

	return &team{
		name:    teamName,
		teamID:  teamID,
		isAdmin: isAdmin,
	}, true
}
