package auth

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"strings"

	jwt "github.com/dgrijalva/jwt-go"
)

func IsAdmin(r *http.Request) bool {
	isAdmin, present := r.Context().Value(isAdminKey).(bool)
	return present && isAdmin
}

func IsSystem(r *http.Request) bool {
	isSystem, present := r.Context().Value(isSystemKey).(bool)
	return present && isSystem
}

func IsAuthenticated(r *http.Request) bool {
	isAuthenticated, _ := r.Context().Value(authenticated).(bool)
	return isAuthenticated
}

func IsAuthorized(r *http.Request) bool {
	authTeam, authTeamFound := GetTeam(r)

	if authTeamFound && authTeam.IsAuthorized(r.URL.Query().Get(":team_name")) {
		return true
	}

	return false
}

func getJWT(r *http.Request, publicKey *rsa.PublicKey) (token *jwt.Token, err error) {
	fun := func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return publicKey, nil
	}

	if ah := r.Header.Get("Authorization"); ah != "" {
		// Should be a bearer token
		if len(ah) > 6 && strings.ToUpper(ah[0:6]) == "BEARER" {
			return jwt.Parse(ah[7:], fun)
		}
	}

	return nil, errors.New("unable to parse authorization header")
}

func GetTeam(r *http.Request) (Team, bool) {
	teamName, namePresent := r.Context().Value(teamNameKey).(string)
	isAdmin, adminPresent := r.Context().Value(isAdminKey).(bool)

	if !(namePresent && adminPresent) {
		return nil, false
	}

	return &team{name: teamName, isAdmin: isAdmin}, true
}

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
