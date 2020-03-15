package accessor

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/mitchellh/mapstructure"
)

//go:generate counterfeiter . Access

type Access interface {
	HasToken() bool
	IsAuthenticated() bool
	IsAuthorized(string) bool
	IsAdmin() bool
	IsSystem() bool
	TeamNames() []string
	CSRFToken() string
	UserName() string
}

type access struct {
	*jwt.Token
	action        string
	actionRoleMap ActionRoleMap
}

func (a *access) HasToken() bool {
	return a.Token != nil
}

func (a *access) IsAuthenticated() bool {
	return a.HasToken() && a.Token.Valid
}

func (a *access) Claims() jwt.MapClaims {
	if a.IsAuthenticated() {
		if claims, ok := a.Token.Claims.(jwt.MapClaims); ok {
			return claims
		}
	}
	return jwt.MapClaims{}
}

func (a *access) IsAuthorized(team string) bool {
	if a.IsAdmin() {
		return true
	}
	for teamName, teamRoles := range a.TeamRoles() {
		if teamName != team {
			continue
		}
		for _, teamRole := range teamRoles {
			if a.hasPermission(teamRole) {
				return true
			}
		}
	}
	return false
}

func (a *access) hasPermission(role string) bool {
	switch a.actionRoleMap.RoleOfAction(a.action) {
	case OwnerRole:
		return role == OwnerRole
	case MemberRole:
		return role == OwnerRole || role == MemberRole
	case OperatorRole:
		return role == OwnerRole || role == MemberRole || role == OperatorRole
	case ViewerRole:
		return role == OwnerRole || role == MemberRole || role == OperatorRole || role == ViewerRole
	default:
		return false
	}
}

func (a *access) IsAdmin() bool {
	if isAdminClaim, ok := a.Claims()["is_admin"]; ok {
		isAdmin, ok := isAdminClaim.(bool)
		return ok && isAdmin
	}
	return false
}

func (a *access) IsSystem() bool {
	if isSystemClaim, ok := a.Claims()[System]; ok {
		isSystem, ok := isSystemClaim.(bool)
		return ok && isSystem
	}
	return false
}

func (a *access) TeamNames() []string {

	teams := []string{}
	for teamName := range a.TeamRoles() {
		teams = append(teams, teamName)
	}

	return teams
}

func (a *access) TeamRoles() map[string][]string {
	teamRoles := map[string][]string{}

	if teamsClaim, ok := a.Claims()["teams"]; ok {

		// support legacy token format with team names array
		if teamsArr, ok := teamsClaim.([]interface{}); ok {
			for _, teamObj := range teamsArr {
				if teamName, ok := teamObj.(string); ok {
					teamRoles[teamName] = []string{OwnerRole}
				}
			}
		} else {
			_ = mapstructure.Decode(teamsClaim, &teamRoles)
		}
	}

	return teamRoles
}

func (a *access) CSRFToken() string {
	if csrfTokenClaim, ok := a.Claims()["csrf"]; ok {
		if csrfToken, ok := csrfTokenClaim.(string); ok {
			return csrfToken
		}
	}
	return ""
}

func (a *access) UserName() string {
	if userName, ok := a.Claims()["user_name"]; ok {
		if userName, ok := userName.(string); ok {
			return userName
		}
	} else if systemName, ok := a.Claims()[System]; systemName == true && ok {
		return System
	}
	return ""
}
