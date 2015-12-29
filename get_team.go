package auth

import (
	"net/http"

	"github.com/gorilla/context"
)

func GetTeam(r *http.Request) (string, int, bool, bool) {
	storedTeamName, namePresent := context.GetOk(r, teamNameKey)
	storedTeamID, idPresent := context.GetOk(r, teamIDKey)
	storedIsAdmin, adminPresent := context.GetOk(r, isAdminKey)

	var teamName string
	var teamID int
	var isAdmin bool
	found := namePresent && idPresent && adminPresent
	if found {
		teamName = storedTeamName.(string)
		teamID = storedTeamID.(int)
		isAdmin = storedIsAdmin.(bool)
	}

	return teamName, teamID, isAdmin, found
}
