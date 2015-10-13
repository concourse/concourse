package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.google.com/p/go.crypto/bcrypt"
)

var ErrUnparsableHeader = errors.New("cannot parse 'Authorization' header")

//go:generate counterfeiter . Validator
type Validator interface {
	IsAuthenticated(*http.Request) bool
	Unauthorized(http.ResponseWriter, *http.Request)
}

type NoopValidator struct{}

func (NoopValidator) IsAuthenticated(*http.Request) bool              { return true }
func (NoopValidator) Unauthorized(http.ResponseWriter, *http.Request) {}

type BasicAuthHashedValidator struct {
	Username       string
	HashedPassword string
}

func (validator BasicAuthHashedValidator) IsAuthenticated(r *http.Request) bool {
	auth := r.Header.Get("Authorization")

	username, password, err := ExtractUsernameAndPassword(auth)
	if err != nil {
		return false
	}

	return validator.correctCredentials(username, password)
}

func (validator BasicAuthHashedValidator) Unauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, "not authorized")
}

func (validator BasicAuthHashedValidator) correctCredentials(username string, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(validator.HashedPassword), []byte(password))
	return validator.Username == username && err == nil
}

type BasicAuthValidator struct {
	Username string
	Password string
}

func (validator BasicAuthValidator) IsAuthenticated(r *http.Request) bool {
	auth := r.Header.Get("Authorization")

	username, password, err := ExtractUsernameAndPassword(auth)
	if err != nil {
		return false
	}

	return validator.correctCredentials(username, password)
}

func (validator BasicAuthValidator) correctCredentials(username string, password string) bool {
	return validator.Username == username && validator.Password == password
}

func ExtractUsernameAndPassword(authorizationHeader string) (string, string, error) {
	if !strings.HasPrefix(authorizationHeader, "Basic ") {
		return "", "", ErrUnparsableHeader
	}

	encodedCredentials := authorizationHeader[6:]
	credentials, err := base64.StdEncoding.DecodeString(encodedCredentials)
	if err != nil {
		return "", "", ErrUnparsableHeader
	}

	parts := strings.Split(string(credentials), ":")
	if len(parts) != 2 {
		return "", "", ErrUnparsableHeader
	}

	return parts[0], parts[1], nil
}
