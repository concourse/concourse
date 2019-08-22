package token

import (
	"errors"
	"net/http"
	"strconv"
	"time"
)

//go:generate counterfeiter . Middleware
type Middleware interface {
	SetToken(http.ResponseWriter, string, time.Time) error
	UnsetToken(http.ResponseWriter)
	GetToken(*http.Request) string
}

type middleware struct {
	secureCookies bool
}

func NewMiddleware(secureCookies bool) Middleware {
	return &middleware{secureCookies: secureCookies}
}

const NumCookies = 15
const authCookieName = "skymarshal_auth"
const maxCookieSize = 4000

func (m *middleware) UnsetToken(w http.ResponseWriter) {
	for i := 0; i < NumCookies; i++ {
		http.SetCookie(w, &http.Cookie{
			Name:     authCookieName + strconv.Itoa(i),
			Path:     "/",
			MaxAge:   -1,
			Secure:   m.secureCookies,
			HttpOnly: true,
		})
	}
}

func (m *middleware) SetToken(w http.ResponseWriter, tokenStr string, expiry time.Time) error {
	tokenLength := len(tokenStr)
	if tokenLength > maxCookieSize*NumCookies {
		return errors.New("token is too long to fit in cookies")
	}

	for i := 0; i < NumCookies; i++ {
		if len(tokenStr) > maxCookieSize {
			http.SetCookie(w, &http.Cookie{
				Name:     authCookieName + strconv.Itoa(i),
				Value:    tokenStr[:maxCookieSize],
				Path:     "/",
				Expires:  expiry,
				HttpOnly: true,
				Secure:   m.secureCookies,
			})
			tokenStr = tokenStr[maxCookieSize:]
		} else {
			http.SetCookie(w, &http.Cookie{
				Name:     authCookieName + strconv.Itoa(i),
				Value:    tokenStr,
				Path:     "/",
				Expires:  expiry,
				HttpOnly: true,
				Secure:   m.secureCookies,
			})
			break
		}
	}
	return nil
}

func (m *middleware) GetToken(r *http.Request) string {
	authCookie := ""
	for i := 0; i < NumCookies; i++ {
		cookie, err := r.Cookie(authCookieName + strconv.Itoa(i))
		if err == nil {
			authCookie += cookie.Value
		}
	}
	return authCookie
}
