package idtoken_test

import (
	"github.com/concourse/concourse/atc/creds/idtoken"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ManagerFactory", func() {
	var factory *idtoken.ManagerFactory
	var signingKeyFactory *dbfakes.FakeSigningKeyFactory
	var config map[string]interface{}

	BeforeEach(func() {
		factory = idtoken.NewManagerFactory().(*idtoken.ManagerFactory)
		signingKeyFactory = &dbfakes.FakeSigningKeyFactory{}
		factory.SetSigningKeyFactory(signingKeyFactory)

		config = map[string]interface{}{
			"audience": []interface{}{"sts.amazonaws.com"},
		}
	})

	Context("when only issuer is set", func() {
		BeforeEach(func() {
			factory.SetIssuer("https://concourse.example.com")
		})

		It("uses issuer for token generation", func() {
			manager, err := factory.NewInstance(config)
			Expect(err).ToNot(HaveOccurred())
			Expect(manager).ToNot(BeNil())

			gen := manager.(*idtoken.Manager).GetTokenGenerator()
			Expect(gen.Issuer).To(Equal("https://concourse.example.com"))
		})
	})

	Context("when both issuer and oidcIssuer are set", func() {
		BeforeEach(func() {
			factory.SetIssuer("https://concourse.example.com")
			factory.SetOIDCIssuer("https://oidc.example.com")
		})

		It("prefers oidcIssuer over issuer", func() {
			manager, err := factory.NewInstance(config)
			Expect(err).ToNot(HaveOccurred())
			Expect(manager).ToNot(BeNil())

			gen := manager.(*idtoken.Manager).GetTokenGenerator()
			Expect(gen.Issuer).To(Equal("https://oidc.example.com"))
		})
	})

	Context("when neither issuer is set", func() {
		It("returns an error", func() {
			_, err := factory.NewInstance(config)
			Expect(err).To(MatchError(ContainSubstring("issuer not set")))
		})
	})
})
