package helpers

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"

	"github.com/concourse/atc/auth"
	. "github.com/onsi/gomega"
	"github.com/sclevine/agouti"
)

func WebLogin(page *agouti.Page, atcURL string) error {
	dev, basicAuth, _, err := getAuthMethods(atcURL)
	if err != nil {
		return err
	}

	if dev {
		return nil
	} else if basicAuth != nil {
		Expect(page.Navigate(atcURL)).To(Succeed())
		basicAuthentication(page, basicAuth.Username, basicAuth.Password)
	}

	return errors.New("Unable to determine authentication")
}

func basicAuthentication(page *agouti.Page, username, password string) {
	header := fmt.Sprintf("%s:%s", username, password)

	page.SetCookie(&http.Cookie{
		Name:  auth.CookieName,
		Value: "Basic " + base64.StdEncoding.EncodeToString([]byte(header)),
	})

	// PhantomJS won't send the cookie on ajax requests if the page is not
	// refreshed
	page.Refresh()
}
