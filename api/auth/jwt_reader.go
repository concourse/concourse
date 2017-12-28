package auth

import (
	"crypto/rsa"
	"net/http"

	jwt "github.com/dgrijalva/jwt-go"
)

//go:generate counterfeiter . UserContextReader
type UserContextReader interface {
	GetTeam(r *http.Request) (string, bool, bool)
	GetSystem(r *http.Request) (bool, bool)
	GetCSRFToken(r *http.Request) (string, bool)
}

type JWTReader struct {
	PublicKey *rsa.PublicKey
}

func (jr JWTReader) GetTeam(r *http.Request) (string, bool, bool) {
	token, err := getJWT(r, jr.PublicKey)
	if err != nil {
		return "", false, false
	}

	claims := token.Claims.(jwt.MapClaims)
	teamNameInterface, teamNameOK := claims[teamNameClaimKey]
	isAdminInterface, isAdminOK := claims[isAdminClaimKey]

	if !(teamNameOK && isAdminOK) {
		return "", false, false
	}

	teamName := teamNameInterface.(string)
	isAdmin := isAdminInterface.(bool)

	return teamName, isAdmin, true
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

func (jr JWTReader) GetCSRFToken(r *http.Request) (string, bool) {
	token, err := getJWT(r, jr.PublicKey)
	if err != nil {
		return "", false
	}

	claims := token.Claims.(jwt.MapClaims)
	csrfToken, ok := claims[csrfTokenClaimKey]
	if !ok {
		return "", false
	}

	return csrfToken.(string), true
}
