package auth

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"code.google.com/p/go.crypto/bcrypt"
)

type Handler struct {
	Handler        http.Handler
	Username       string
	HashedPassword string
}

const CookieName = "ATC-Authorization"

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
