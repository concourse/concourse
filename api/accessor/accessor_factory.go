package accessor

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"strings"

	jwt "github.com/dgrijalva/jwt-go"
)

//go:generate counterfeiter . AccessFactory

type AccessFactory interface {
	Create(*http.Request) Access
}

type accessFactory struct {
	publicKey *rsa.PublicKey
}

func NewAccessFactory(key *rsa.PublicKey) AccessFactory {
	return &accessFactory{
		publicKey: key,
	}
}

func (a *accessFactory) Create(r *http.Request) Access {
	var token *jwt.Token
	var err error
	token, err = a.parseToken(r)
	if err != nil {
		token = &jwt.Token{}
	}
	return &access{
		token,
	}
}

func (a *accessFactory) parseToken(r *http.Request) (*jwt.Token, error) {
	fun := func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return a.publicKey, nil
	}

	if ah := r.Header.Get("Authorization"); ah != "" {
		// Should be a bearer token
		if len(ah) > 6 && strings.ToUpper(ah[0:6]) == "BEARER" {
			return jwt.Parse(ah[7:], fun)
		}
	}

	return nil, errors.New("unable to parse authorization header")
}
