package auth_test

import (
	"encoding/base64"

	"github.com/concourse/atc/auth"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AccessTokenGenerator", func() {
	var tokenGenerator auth.AccessTokenGenerator

	BeforeEach(func() {
		tokenGenerator = auth.NewAccessTokenGenerator()
	})

	Describe("GenerateToken", func() {
		It("sets no arguments", func() {
			tokenType, tokenValue, err := tokenGenerator.GenerateToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(string(tokenType)).To(Equal("Access"))
			_, err = base64.URLEncoding.DecodeString(string(tokenValue))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
