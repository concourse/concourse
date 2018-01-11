package auth

import (
	"crypto/rsa"
	"net/http"
)

//go:generate counterfeiter . TokenValidator

type TokenValidator interface {
	IsAuthenticated(r *http.Request) bool
}

type JWTValidator struct {
	PublicKey *rsa.PublicKey
}

func (validator JWTValidator) IsAuthenticated(r *http.Request) bool {
	token, err := getJWT(r, validator.PublicKey)
	if err != nil {
		return false
	}

	return token.Valid
}
