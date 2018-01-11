package auth_test

import (
	"github.com/concourse/skymarshal/auth"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CsrfTokenGenerator", func() {
	var tokenGenerator auth.CSRFTokenGenerator

	BeforeEach(func() {
		tokenGenerator = auth.NewCSRFTokenGenerator()
	})

	It("returns token of length 32 bytes", func() {
		token, err := tokenGenerator.GenerateToken()
		Expect(err).NotTo(HaveOccurred())
		Expect(token).To(HaveLen(64))
	})

	It("returns a lowercase hexbytes string that is web safe", func() {
		token, err := tokenGenerator.GenerateToken()
		Expect(err).NotTo(HaveOccurred())
		Expect(token).To(MatchRegexp("^[a-f0-9]{64}$"))
	})
})
