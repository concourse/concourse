package token

import (
	"errors"
	"net/http"
	"strconv"
	"time"
)

//go:generate counterfeiter . Middleware
type Middleware interface {
	SetAuthToken(http.ResponseWriter, string, time.Time) error
	UnsetAuthToken(http.ResponseWriter)
	GetAuthToken(*http.Request) string

	SetCSRFToken(http.ResponseWriter, string, time.Time) error
	UnsetCSRFToken(http.ResponseWriter)
	GetCSRFToken(*http.Request) string

	SetStateToken(http.ResponseWriter, string, time.Time) error
	UnsetStateToken(http.ResponseWriter)
	GetStateToken(*http.Request) string
}

type middleware struct {
	secureCookies bool
}

func NewMiddleware(secureCookies bool) Middleware {
	return &middleware{secureCookies: secureCookies}
}

const NumCookies = 15
const stateCookieName = "skymarshal_state"
const authCookieName = "skymarshal_auth"
const csrfCookieName = "skymarshal_csrf"
const maxCookieSize = 4000

func (m *middleware) UnsetAuthToken(w http.ResponseWriter) {
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

func (m *middleware) SetAuthToken(w http.ResponseWriter, tokenStr string, expiry time.Time) error {
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

func (m *middleware) GetAuthToken(r *http.Request) string {
	authCookie := ""
	for i := 0; i < NumCookies; i++ {
		cookie, err := r.Cookie(authCookieName + strconv.Itoa(i))
		if err == nil {
			authCookie += cookie.Value
		}
	}
	return authCookie
}

func (m *middleware) UnsetCSRFToken(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Path:     "/",
		MaxAge:   -1,
		Secure:   m.secureCookies,
		HttpOnly: true,
	})
}

func (m *middleware) SetCSRFToken(w http.ResponseWriter, csrfToken string, expiry time.Time) error {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    csrfToken,
		Path:     "/",
		Expires:  expiry,
		Secure:   m.secureCookies,
		HttpOnly: true,
	})

	return nil
}

func (m *middleware) GetCSRFToken(r *http.Request) string {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (m *middleware) UnsetStateToken(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Path:     "/",
		MaxAge:   -1,
		Secure:   m.secureCookies,
		HttpOnly: true,
	})
}

func (m *middleware) SetStateToken(w http.ResponseWriter, stateToken string, expiry time.Time) error {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    stateToken,
		Path:     "/",
		Expires:  expiry,
		Secure:   m.secureCookies,
		HttpOnly: true,
	})

	return nil
}

func (m *middleware) GetStateToken(r *http.Request) string {
	cookie, err := r.Cookie(stateCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
