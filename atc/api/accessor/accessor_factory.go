package accessor

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"strings"

	jwt "github.com/dgrijalva/jwt-go"
)

//go:generate counterfeiter . AccessFactory

type AccessFactory interface {
	Create(*http.Request, string) Access
}

type accessFactory struct {
	publicKey *rsa.PublicKey
}

func NewAccessFactory(key *rsa.PublicKey) AccessFactory {
	return &accessFactory{
		publicKey: key,
	}
}

func (a *accessFactory) Create(r *http.Request, action string) Access {

	header := r.Header.Get("Authorization")
	if header == "" {
		return &access{nil, action}
	}

	if len(header) < 7 || strings.ToUpper(header[0:6]) != "BEARER" {
		return &access{&jwt.Token{}, action}
	}

	token, err := jwt.Parse(header[7:], a.validate)
	if err != nil {
		return &access{&jwt.Token{}, action}
	}

	return &access{token, action}
}

func (a *accessFactory) validate(token *jwt.Token) (interface{}, error) {

	if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
		return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
	}

	return a.publicKey, nil
}
