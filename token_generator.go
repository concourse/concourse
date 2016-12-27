package tsa

import (
	"crypto/rsa"
	"time"

	"github.com/dgrijalva/jwt-go"
)

//go:generate counterfeiter . TokenGenerator
type TokenGenerator interface {
	GenerateSystemToken() (string, error)
	GenerateTeamToken(teamName string) (string, error)
}

type tokenGenerator struct {
	signingKey *rsa.PrivateKey
}

func NewTokenGenerator(signingKey *rsa.PrivateKey) TokenGenerator {
	return &tokenGenerator{signingKey: signingKey}
}

func (tk *tokenGenerator) GenerateSystemToken() (string, error) {
	exp := time.Now().Add(time.Hour)
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"exp":    exp.Unix(),
		"system": true,
	})

	return jwtToken.SignedString(tk.signingKey)
}

func (tk *tokenGenerator) GenerateTeamToken(teamName string) (string, error) {
	exp := time.Now().Add(time.Hour)
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"exp":      exp.Unix(),
		"teamName": teamName,
		"isAdmin":  false,
	})

	return jwtToken.SignedString(tk.signingKey)
}
