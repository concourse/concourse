package auth

import (
	"crypto/rsa"
	"time"

	"github.com/dgrijalva/jwt-go"
)

//go:generate counterfeiter . AuthTokenGenerator

type TokenType string
type TokenValue string

const TokenTypeBearer = "Bearer"
const expClaimKey = "exp"
const teamNameClaimKey = "teamName"
const isAdminClaimKey = "isAdmin"
const csrfTokenClaimKey = "csrf"
const isSystemClaimKey = "system"

type AuthTokenGenerator interface {
	GenerateToken(expiration time.Time, teamName string, isAdmin bool, csrfToken string) (TokenType, TokenValue, error)
}

type authTokenGenerator struct {
	privateKey *rsa.PrivateKey
}

func NewAuthTokenGenerator(privateKey *rsa.PrivateKey) AuthTokenGenerator {
	return &authTokenGenerator{
		privateKey: privateKey,
	}
}

func (generator *authTokenGenerator) GenerateToken(expiration time.Time, teamName string, isAdmin bool, csrfToken string) (TokenType, TokenValue, error) {
	jwtToken := jwt.NewWithClaims(SigningMethod, jwt.MapClaims{
		expClaimKey:       expiration.Unix(),
		teamNameClaimKey:  teamName,
		isAdminClaimKey:   isAdmin,
		csrfTokenClaimKey: csrfToken,
	})

	signed, err := jwtToken.SignedString(generator.privateKey)
	if err != nil {
		return "", "", err
	}

	return TokenTypeBearer, TokenValue(signed), err
}
