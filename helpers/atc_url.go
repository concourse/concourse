package helpers

import (
	"net/http"
	"os"

	"code.cloudfoundry.org/gunk/urljoiner"
)

const defaultAtcURL = "http://10.244.15.2:8080"

var storedAtcURL *string

func AtcURL() string {
	if storedAtcURL != nil {
		return *storedAtcURL
	}

	atcURL := os.Getenv("ATC_URL")
	if atcURL == "" {
		response, err := http.Get(urljoiner.Join(defaultAtcURL, "api/v1/auth/methods"))
		if err != nil || response.StatusCode != http.StatusOK {
			panic("must set $ATC_URL")
		}

		atcURL = defaultAtcURL
	}

	storedAtcURL = &atcURL
	return atcURL
}
