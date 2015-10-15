package auth

import (
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

type BasicAuthHashedValidator struct {
	Username       string
	HashedPassword string
}

func (validator BasicAuthHashedValidator) IsAuthenticated(r *http.Request) bool {
	auth := r.Header.Get("Authorization")

	username, password, err := extractUsernameAndPassword(auth)
	if err != nil {
		return false
	}

	return validator.correctCredentials(username, password)
}

func (validator BasicAuthHashedValidator) correctCredentials(username string, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(validator.HashedPassword), []byte(password))
	return validator.Username == username && err == nil
}
