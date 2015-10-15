package auth

import (
	"crypto/rsa"
	"fmt"
	"net/http"

	"github.com/dgrijalva/jwt-go"
)

type JWTValidator struct {
	PublicKey *rsa.PublicKey
}

func (validator JWTValidator) IsAuthenticated(r *http.Request) bool {
	token, err := jwt.ParseFromRequest(r, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return validator.PublicKey, nil
	})
	if err != nil {
		return false
	}

	return token.Valid
}
