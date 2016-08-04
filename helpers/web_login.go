package helpers

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/gunk/urljoiner"
	"github.com/concourse/atc/auth"
	. "github.com/onsi/gomega"
	"github.com/sclevine/agouti"
	. "github.com/sclevine/agouti/matchers"
)

func WebLogin(page *agouti.Page, atcURL string) error {
	noAuth, basicAuth, oauth, err := GetAuthMethods(atcURL)
	if err != nil {
		return err
	}

	switch {
	case noAuth || basicAuth != nil:
		Expect(page.Navigate(atcURL)).To(Succeed())
		return basicAuthenticationWeb(page, atcURL)
	case oauth != nil:
		return oauthAuthenticationWeb(page, oauth, atcURL)
	}

	return errors.New("Unable to determine authentication")
}

func basicAuthenticationWeb(page *agouti.Page, atcURL string) error {
	token, err := GetATCToken(atcURL)
	if err != nil {
		return err
	}

	page.SetCookie(&http.Cookie{
		Name:  auth.CookieName,
		Value: string(token.Type) + " " + string(token.Value),
	})

	// PhantomJS won't send the cookie on ajax requests if the page is not
	// refreshed
	return page.Refresh()
}

func oauthAuthenticationWeb(page *agouti.Page, oauth *oauthAuthCredentials, atcURL string) error {
	switch oauth.Provider {
	case githubProvider:
		err := loginToGithub(oauth, page)
		if err != nil {
			return err
		}

		Expect(page.Navigate(urljoiner.Join(atcURL, "login"))).To(Succeed())
		if err != nil {
			return err
		}

		loginLink := page.FindByLink("Log in with GitHub")
		Eventually(loginLink).Should(BeFound())
		return loginLink.Click()
	}

	return errors.New("Unable to login, oauth provider not identified")
}
