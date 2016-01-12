package helpers

import "github.com/sclevine/agouti"

func Screenshot(page *agouti.Page) {
	page.Screenshot("/tmp/screenshot.png")
}
