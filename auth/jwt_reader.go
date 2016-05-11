package auth

import (
	"crypto/rsa"
	"net/http"
)

type JWTReader struct {
	PublicKey *rsa.PublicKey
}

func (jr JWTReader) GetTeam(r *http.Request) (string, int, bool, bool) {
	token, err := getJWT(r, jr.PublicKey)
	if err != nil {
		return "", 0, false, false
	}

	teamNameInterface, teamNameOK := token.Claims[teamNameClaimKey]
	teamIDInterface, teamIDOK := token.Claims[teamIDClaimKey]
	isAdminInterface, isAdminOK := token.Claims[isAdminClaimKey]

	if !(teamNameOK && teamIDOK && isAdminOK) {
		return "", 0, false, false
	}

	teamName := teamNameInterface.(string)
	teamID := int(teamIDInterface.(float64))
	isAdmin := isAdminInterface.(bool)

	return teamName, teamID, isAdmin, true
}
