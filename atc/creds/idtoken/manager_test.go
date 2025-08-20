package idtoken_test

import (
	"time"

	"github.com/concourse/concourse/atc/creds/idtoken"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/go-jose/go-jose/v4"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IDToken Manager", func() {

	var signingKeyFactory db.SigningKeyFactory
	var config map[string]any

	BeforeEach(func() {
		signingKeyFactory = &dbfakes.FakeSigningKeyFactory{}
		config = map[string]any{
			"audience":      []any{"testaud"},
			"subject_scope": string(idtoken.SubjectScopeTeam),
			"expires_in":    "15m",
			"algorithm":     "ES256",
		}
	})

	It("accepts valid configuration", func() {
		manager, err := idtoken.NewManager(testIssuer, signingKeyFactory, config)
		Expect(err).ToNot(HaveOccurred())
		Expect(manager.Init(nil)).To(Succeed())
		Expect(manager.Validate()).To(Succeed())

		gen := manager.GetTokenGenerator()

		Expect(gen.Issuer).To(Equal(testIssuer))
		Expect(gen.SubjectScope).To(Equal(idtoken.SubjectScopeTeam))
		Expect(gen.Audience).To(ContainElement("testaud"))
		Expect(gen.ExpiresIn).To(Equal(15 * time.Minute))
		Expect(gen.Algorithm).To(Equal(jose.ES256))
	})

	It("rejects malformed audience", func() {
		config["audience"] = "invalid"
		_, err := idtoken.NewManager(testIssuer, signingKeyFactory, config)
		Expect(err).To(HaveOccurred())
	})

	It("rejects malformed subject_scope", func() {
		config["subject_scope"] = 123
		_, err := idtoken.NewManager(testIssuer, signingKeyFactory, config)
		Expect(err).To(HaveOccurred())
	})

	It("rejects malformed expires_in", func() {
		config["expires_in"] = 123
		_, err := idtoken.NewManager(testIssuer, signingKeyFactory, config)
		Expect(err).To(HaveOccurred())

		config["expires_in"] = "15abc"
		_, err = idtoken.NewManager(testIssuer, signingKeyFactory, config)
		Expect(err).To(HaveOccurred())
	})

	It("rejects malformed algorithm", func() {
		config["algorithm"] = 123
		_, err := idtoken.NewManager(testIssuer, signingKeyFactory, config)
		Expect(err).To(HaveOccurred())
	})

	It("rejects unknown settings", func() {
		config["unknown"] = "abc"
		_, err := idtoken.NewManager(testIssuer, signingKeyFactory, config)
		Expect(err).To(HaveOccurred())
	})

	It("rejects unknown subject_scope", func() {
		config["subject_scope"] = "something"
		manager, err := idtoken.NewManager(testIssuer, signingKeyFactory, config)
		Expect(err).ToNot(HaveOccurred())

		Expect(manager.Validate()).ToNot(Succeed())
	})

	It("rejects expires_in if larger then 24h", func() {
		config["expires_in"] = "48h"
		manager, err := idtoken.NewManager(testIssuer, signingKeyFactory, config)
		Expect(err).ToNot(HaveOccurred())

		Expect(manager.Validate()).ToNot(Succeed())
	})

	It("rejects unknown algorithm", func() {
		config["algorithm"] = "something"
		manager, err := idtoken.NewManager(testIssuer, signingKeyFactory, config)
		Expect(err).ToNot(HaveOccurred())

		Expect(manager.Validate()).ToNot(Succeed())
	})

})
