package tsa

import (
	"crypto/rsa"
	"time"

	"github.com/dgrijalva/jwt-go"
)

//go:generate counterfeiter . TokenGenerator
type TokenGenerator interface {
	GenerateToken() (string, error)
}

type tokenGenerator struct {
	signingKey *rsa.PrivateKey
}

func NewTokenGenerator(signingKey *rsa.PrivateKey) TokenGenerator {
	return &tokenGenerator{signingKey: signingKey}
}

func (tk *tokenGenerator) GenerateToken() (string, error) {
	exp := time.Now().Add(time.Hour)
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"exp":    exp.Unix(),
		"system": true,
	})

	return jwtToken.SignedString(tk.signingKey)
}
