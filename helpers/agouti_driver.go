package helpers

import (
	"os"

	"github.com/sclevine/agouti"
)

func AgoutiDriver() *agouti.WebDriver {
	if os.Getenv("FORCE_SELENIUM") == "true" {
		return agouti.Selenium(agouti.Browser("firefox"))
	} else {
		return agouti.ChromeDriver()
	}
}
