package auth

import (
	"crypto/rsa"
	"net/http"

	jwt "github.com/dgrijalva/jwt-go"
)

type JWTReader struct {
	PublicKey *rsa.PublicKey
}

func (jr JWTReader) GetTeam(r *http.Request) (string, int, bool, bool) {
	token, err := getJWT(r, jr.PublicKey)
	if err != nil {
		return "", 0, false, false
	}

	claims := token.Claims.(jwt.MapClaims)
	teamNameInterface, teamNameOK := claims[teamNameClaimKey]
	teamIDInterface, teamIDOK := claims[teamIDClaimKey]
	isAdminInterface, isAdminOK := claims[isAdminClaimKey]

	if !(teamNameOK && teamIDOK && isAdminOK) {
		return "", 0, false, false
	}

	teamName := teamNameInterface.(string)
	teamID := int(teamIDInterface.(float64))
	isAdmin := isAdminInterface.(bool)

	return teamName, teamID, isAdmin, true
}

func (jr JWTReader) GetSystem(r *http.Request) (bool, bool) {
	token, err := getJWT(r, jr.PublicKey)
	if err != nil {
		return false, false
	}

	claims := token.Claims.(jwt.MapClaims)
	isSystemInterface, isSystemOK := claims[isSystemKey]
	if !isSystemOK {
		return false, false
	}

	return isSystemInterface.(bool), true
}
