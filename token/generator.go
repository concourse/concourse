package token

import (
	"crypto/rsa"
	"errors"

	"github.com/dgrijalva/jwt-go"
	"golang.org/x/oauth2"
)

//go:generate counterfeiter . Generator
type Generator interface {
	Generate(map[string]interface{}) (*oauth2.Token, error)
}

func NewGenerator(signingKey *rsa.PrivateKey) *generator {
	return &generator{
		SigningKey: signingKey,
	}
}

type generator struct {
	SigningKey *rsa.PrivateKey
}

func (self *generator) Generate(claims map[string]interface{}) (*oauth2.Token, error) {

	if self.SigningKey == nil {
		return nil, errors.New("Invalid signing key")
	}

	if len(claims) == 0 {
		return nil, errors.New("Invalid claims")
	}

	jwtClaims := jwt.MapClaims(claims)
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwtClaims)

	signedToken, err := jwtToken.SignedString(self.SigningKey)
	if err != nil {
		return nil, err
	}

	oauth2Token := &oauth2.Token{
		TokenType:   "Bearer",
		AccessToken: signedToken,
	}

	return oauth2Token.WithExtra(claims), nil
}
