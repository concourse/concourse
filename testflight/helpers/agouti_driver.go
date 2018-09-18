package helpers

import "github.com/sclevine/agouti"

func AgoutiDriver() *agouti.WebDriver {
	return agouti.PhantomJS()
}
