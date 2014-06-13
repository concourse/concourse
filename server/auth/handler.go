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
	username, password, err := ExtractUsernameAndPassword(r.Header.Get("Authorization"))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(h.HashedPassword), []byte(password))
	if username == h.Username && err == nil {
		h.Handler.ServeHTTP(w, r)
	} else {
		w.WriteHeader(http.StatusUnauthorized)
	}
}

var ErrUnparsableHeader = errors.New("unparseable Authorization header")

func ExtractUsernameAndPassword(authorizationHeader string) (string, string, error) {
	if !strings.HasPrefix(authorizationHeader, "Basic ") {
		return "", "", ErrUnparsableHeader
	}

	substring := authorizationHeader[6:]
	decodedSubstring, err := base64.StdEncoding.DecodeString(substring)
	if err != nil {
		return "", "", ErrUnparsableHeader
	}

	parts := strings.Split(string(decodedSubstring), ":")
	if len(parts) != 2 {
		return "", "", ErrUnparsableHeader
	}

	return parts[0], parts[1], nil
}
