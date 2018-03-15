package accessor

import (
	jwt "github.com/dgrijalva/jwt-go"
)

//go:generate counterfeiter . Access

type Access interface {
	IsAuthenticated() bool
	IsAuthorized(string) bool
	IsAdmin() bool
	IsSystem() bool
	TeamNames() []string
	CSRFToken() string
}

type access struct {
	*jwt.Token
}

func (a *access) IsAuthenticated() bool {
	return a.Token.Valid
}

func (a *access) IsAuthorized(team string) bool {
	for _, teamName := range a.TeamNames() {
		if teamName == team {
			return true
		}
	}
	return false
}

func (a *access) IsAdmin() bool {
	if claims, ok := a.Token.Claims.(jwt.MapClaims); ok {
		if isAdminClaim, ok := claims["isAdmin"]; ok {
			isAdmin, ok := isAdminClaim.(bool)
			return ok && isAdmin
		}
	}
	return false
}

func (a *access) IsSystem() bool {
	if claims, ok := a.Token.Claims.(jwt.MapClaims); ok {
		if isSystemClaim, ok := claims["system"]; ok {
			isSystem, ok := isSystemClaim.(bool)
			return ok && isSystem
		}
	}
	return false
}

func (a *access) TeamNames() []string {
	if claims, ok := a.Token.Claims.(jwt.MapClaims); ok {
		if teamNameClaim, ok := claims["teamName"]; ok {
			if teamName, ok := teamNameClaim.(string); ok {
				return []string{teamName}
			}
		}
	}
	return []string{}
}

func (a *access) CSRFToken() string {
	if claims, ok := a.Token.Claims.(jwt.MapClaims); ok {
		if csrfTokenClaim, ok := claims["csrf"]; ok {
			if csrfToken, ok := csrfTokenClaim.(string); ok {
				return csrfToken
			}
		}
	}
	return ""
}
