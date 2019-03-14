package token

import (
	"crypto/rsa"
	"errors"
	"time"

	"golang.org/x/oauth2"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

//go:generate counterfeiter . Generator
type Generator interface {
	Generate(map[string]interface{}) (*oauth2.Token, error)
}

func NewGenerator(signingKey *rsa.PrivateKey) Generator {
	return &generator{
		SigningKey: signingKey,
	}
}

type generator struct {
	SigningKey *rsa.PrivateKey
}

func (gen *generator) Generate(claims map[string]interface{}) (*oauth2.Token, error) {

	if gen.SigningKey == nil {
		return nil, errors.New("Invalid signing key")
	}

	if len(claims) == 0 {
		return nil, errors.New("Invalid claims")
	}

	signerKey := jose.SigningKey{
		Algorithm: jose.RS256,
		Key:       gen.SigningKey,
	}

	options := &jose.SignerOptions{}
	options = options.WithType("JWT")

	signer, err := jose.NewSigner(signerKey, options)
	if err != nil {
		return nil, err
	}

	signedToken, err := jwt.Signed(signer).Claims(claims).CompactSerialize()
	if err != nil {
		return nil, err
	}

	var expiry time.Time

	exp, ok := claims["exp"].(int64)
	if ok {
		expiry = time.Unix(exp, 0)
	} else {
		expiry = time.Now().Add(24 * time.Hour)
	}

	oauth2Token := &oauth2.Token{
		TokenType:   "Bearer",
		AccessToken: signedToken,
		Expiry:      expiry,
	}

	return oauth2Token.WithExtra(claims), nil
}
