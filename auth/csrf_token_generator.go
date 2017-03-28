package auth

import (
	"crypto/rand"
	"encoding/hex"
)

//go:generate counterfeiter . CSRFTokenGenerator

type CSRFTokenGenerator interface {
	GenerateToken() (string, error)
}

type csrfTokenGenerator struct {
}

func NewCSRFTokenGenerator() CSRFTokenGenerator {
	return &csrfTokenGenerator{}
}

func (generator *csrfTokenGenerator) GenerateToken() (string, error) {
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)

	if err != nil {
		return "", err
	}

	return hex.EncodeToString(randomBytes), nil
}
