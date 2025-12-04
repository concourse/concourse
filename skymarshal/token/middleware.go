package token

import (
	"net/http"
	"time"
)

//counterfeiter:generate . Middleware
type Middleware interface {
	SetAuthToken(http.ResponseWriter, string, time.Time) error
	UnsetAuthToken(http.ResponseWriter)
	GetAuthToken(*http.Request) string

	SetCSRFToken(http.ResponseWriter, string, time.Time) error
	UnsetCSRFToken(http.ResponseWriter)
	GetCSRFToken(*http.Request) string
}

type middleware struct {
	secureCookies bool
}

func NewMiddleware(secureCookies bool) Middleware {
	return &middleware{secureCookies: secureCookies}
}

const authCookieName = "skymarshal_auth"
const csrfCookieName = "skymarshal_csrf"

func (m *middleware) UnsetAuthToken(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Path:     "/",
		MaxAge:   -1,
		Secure:   m.secureCookies,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *middleware) SetAuthToken(w http.ResponseWriter, tokenStr string, expiry time.Time) error {
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    tokenStr,
		Path:     "/",
		Expires:  expiry,
		HttpOnly: true,
		Secure:   m.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})

	return nil
}

func (m *middleware) GetAuthToken(r *http.Request) string {
	cookie, err := r.Cookie(authCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (m *middleware) UnsetCSRFToken(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Path:     "/",
		MaxAge:   -1,
		Secure:   m.secureCookies,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
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
		SameSite: http.SameSiteLaxMode,
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
