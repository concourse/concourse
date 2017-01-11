package auth

import (
	"crypto/rsa"
	"time"

	"github.com/dgrijalva/jwt-go"
)

//go:generate counterfeiter . TokenGenerator

type TokenType string
type TokenValue string

const TokenTypeBearer = "Bearer"
const expClaimKey = "exp"
const teamNameClaimKey = "teamName"
const isAdminClaimKey = "isAdmin"

type TokenGenerator interface {
	GenerateToken(expiration time.Time, teamName string, isAdmin bool) (TokenType, TokenValue, error)
}

type tokenGenerator struct {
	privateKey *rsa.PrivateKey
}

func NewTokenGenerator(privateKey *rsa.PrivateKey) TokenGenerator {
	return &tokenGenerator{
		privateKey: privateKey,
	}
}

func (generator *tokenGenerator) GenerateToken(expiration time.Time, teamName string, isAdmin bool) (TokenType, TokenValue, error) {
	jwtToken := jwt.NewWithClaims(SigningMethod, jwt.MapClaims{
		expClaimKey:      expiration.Unix(),
		teamNameClaimKey: teamName,
		isAdminClaimKey:  isAdmin,
	})

	signed, err := jwtToken.SignedString(generator.privateKey)
	if err != nil {
		return "", "", err
	}

	return TokenTypeBearer, TokenValue(signed), err
}
