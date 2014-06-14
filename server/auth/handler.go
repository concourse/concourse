package auth

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"code.google.com/p/go.crypto/bcrypt"
)

type Handler struct {
	Handler        http.Handler
	Username       string
	HashedPassword string
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	username, password, err := ExtractUsernameAndPassword(header)
	if err != nil {
		h.unauthorized(w)
		return
	}

	if h.correctCredentials(username, password) {
		h.Handler.ServeHTTP(w, r)
	} else {
		h.unauthorized(w)
	}
}

func (h Handler) correctCredentials(username string, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(h.HashedPassword), []byte(password))
	return h.Username == username && err == nil
}

func (h Handler) unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
	w.WriteHeader(http.StatusUnauthorized)
}

var ErrUnparsableHeader = errors.New("cannot parse 'Authorization' header")

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
