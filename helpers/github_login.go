package helpers

import (
	"code.cloudfoundry.org/gunk/urljoiner"
	. "github.com/onsi/gomega"
	"github.com/sclevine/agouti"
	. "github.com/sclevine/agouti/matchers"
)

func getGithubOauthToken(atcURL string, creds *oauthAuthCredentials, page *agouti.Page) (string, error) {
	err := loginToGithub(creds, page)
	if err != nil {
		return "", err
	}

	err = page.Navigate(urljoiner.Join(atcURL, "auth/github"))
	if err != nil {
		return "", err
	}

	token, err := page.Find("body").Text()
	if err != nil {
		return "", err
	}

	return token, nil
}

func loginToGithub(creds *oauthAuthCredentials, page *agouti.Page) error {
	Expect(page.Navigate("https://github.com/login")).To(Succeed())

	Eventually(page.FindByName("login")).Should(BeFound())

	err := page.FindByName("login").Fill(creds.Username)
	if err != nil {
		return err
	}
	err = page.FindByName("password").Fill(creds.Password)
	if err != nil {
		return err
	}

	err = page.FindByName("commit").Click()
	if err != nil {
		return err
	}

	return nil
}
