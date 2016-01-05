package auth

import (
	"crypto/rsa"
	"net/http"
)

type JWTReader struct {
	PublicKey *rsa.PublicKey
}

func (jr JWTReader) GetTeam(r *http.Request) (string, int, bool, bool) {
	var teamName string
	var teamID int
	var isAdmin bool

	token, err := getJWT(r, jr.PublicKey)
	if err != nil {
		return teamName, teamID, isAdmin, false
	}

	teamNameInterface, teamNameOK := token.Claims[teamNameClaimKey]
	teamIDInterface, teamIDOK := token.Claims[teamIDClaimKey]
	isAdminInterface, isAdminOK := token.Claims[isAdminClaimKey]

	found := teamNameOK && teamIDOK && isAdminOK

	if found {
		teamName = teamNameInterface.(string)
		teamID = int(teamIDInterface.(float64))
		isAdmin = isAdminInterface.(bool)
	}

	return teamName, teamID, isAdmin, found
}
