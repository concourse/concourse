package auth

import (
	"crypto/sha512"

	"encoding/base64"

	uuid "github.com/satori/go.uuid"
)

type AccessTokenValue string

//go:generate counterfeiter . AccessTokenGenerator
const tokenTypeAccess = "Access"

// AccessTokenGenerator generates an access_token to be used with webhooks
type AccessTokenGenerator interface {
	GenerateToken() (TokenType, AccessTokenValue, error)
}

type accessTokenGenerator struct {
}

// NewAccessTokenGenerator returns a new token generator to create access tokens for use with webhooks
func NewAccessTokenGenerator() AccessTokenGenerator {
	return &accessTokenGenerator{}
}

func (generator *accessTokenGenerator) GenerateToken() (TokenType, AccessTokenValue, error) {
	hasher := sha512.New()
	hasher.Write(uuid.NewV1().Bytes())
	return tokenTypeAccess, AccessTokenValue(base64.URLEncoding.EncodeToString(hasher.Sum(nil))), nil
}
