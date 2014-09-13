package auth

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"code.google.com/p/go.crypto/bcrypt"
)

var ErrUnparsableHeader = errors.New("cannot parse 'Authorization' header")

type Validator interface {
	IsAuthenticated(http.ResponseWriter, *http.Request) bool
}

type NoopValidator struct{}

func (NoopValidator) IsAuthenticated(http.ResponseWriter, *http.Request) bool {
	return true
}

type BasicAuthValidator struct {
	Username       string
	HashedPassword string
}

func (validator BasicAuthValidator) IsAuthenticated(w http.ResponseWriter, r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		cookie, err := r.Cookie(CookieName)
		if err == nil {
			auth = cookie.Value
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:    CookieName,
		Value:   auth,
		Path:    "/",
		Expires: time.Now().Add(1 * time.Minute),
	})

	username, password, err := ExtractUsernameAndPassword(auth)
	if err != nil {
		return false
	}

	return validator.correctCredentials(username, password)
}

func (validator BasicAuthValidator) correctCredentials(username string, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(validator.HashedPassword), []byte(password))
	return validator.Username == username && err == nil
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
