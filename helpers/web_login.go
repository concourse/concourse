package helpers

import (
	"net/http"

	"github.com/concourse/atc/auth"
	. "github.com/onsi/gomega"
	"github.com/sclevine/agouti"
)

func WebLogin(page *agouti.Page, atcURL string) error {
	Expect(page.Navigate(atcURL)).To(Succeed())
	return basicAuthenticationWeb(page, atcURL)
}

func basicAuthenticationWeb(page *agouti.Page, atcURL string) error {
	authToken, csrfToken, err := GetATCToken(atcURL)
	if err != nil {
		return err
	}

	page.SetCookie(&http.Cookie{
		Name:  auth.AuthCookieName,
		Value: string(authToken.Type) + " " + string(authToken.Value),
	})

	err = page.RunScript("return localStorage.setItem('csrf_token', '"+csrfToken+"')", nil, nil)
	if err != nil {
		return err
	}

	// PhantomJS won't send the cookie on ajax requests if the page is not
	// refreshed
	return page.Refresh()
}
