package auth

import (
	"crypto/rsa"
	"fmt"
	"net/http"

	"github.com/dgrijalva/jwt-go"
)

type JWTReader struct {
	PublicKey *rsa.PublicKey
}

func (jr JWTReader) GetTeam(r *http.Request) (string, int, bool, bool) {
	var teamName string
	var teamID int
	var isAdmin bool

	token, err := jwt.ParseFromRequest(CopyRequest(r), func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return jr.PublicKey, nil
	})

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
