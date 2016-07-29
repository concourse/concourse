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
	jwtToken := jwt.New(jwt.SigningMethodRS256)
	exp := time.Now().Add(time.Hour)
	jwtToken.Claims["exp"] = exp.Unix()
	jwtToken.Claims["system"] = true
	signedToken, err := jwtToken.SignedString(tk.signingKey)
	return signedToken, err
}
