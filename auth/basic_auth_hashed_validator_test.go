package auth_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/bcrypt"

	"github.com/concourse/atc/auth"
)

var _ = Describe("BasicAuthHashedValidator", func() {
	basicAuthFlow(func(username string, password string) auth.Validator {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
		Expect(err).NotTo(HaveOccurred())

		return auth.BasicAuthHashedValidator{
			Username:       username,
			HashedPassword: string(hashedPassword),
		}
	})
})
