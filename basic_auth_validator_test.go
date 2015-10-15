package auth_test

import (
	. "github.com/onsi/ginkgo"

	"github.com/concourse/atc/auth"
)

var _ = Describe("BasicAuthValidator", func() {
	basicAuthFlow(func(username string, password string) auth.Validator {
		return auth.BasicAuthValidator{
			Username: username,
			Password: password,
		}
	})
})
