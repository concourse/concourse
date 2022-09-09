package conjur_test

import (
	"github.com/concourse/concourse/atc/creds/conjur"
	"github.com/jessevdk/go-flags"

	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manager", func() {
	var manager conjur.Manager

	Describe("IsConfigured()", func() {
		JustBeforeEach(func() {
			_, err := flags.ParseArgs(&manager, []string{})
			Expect(err).To(BeNil())
		})

		It("fails on empty Manager", func() {
			Expect(manager.IsConfigured()).To(BeFalse())
		})

		It("passes if ConjurApplianceURL is set", func() {
			manager.ConjurApplianceUrl = "http://conjur-test"
			Expect(manager.IsConfigured()).To(BeTrue())
		})
	})

	Describe("Validate()", func() {
		JustBeforeEach(func() {
			manager = conjur.Manager{
				ConjurApplianceUrl: "http://conjur-test",
				ConjurAccount:      "account",
				ConjurAuthnLogin:   "login",
				ConjurAuthnApiKey:  "apiKey",
			}
			_, err := flags.ParseArgs(&manager, []string{})
			Expect(err).To(BeNil())
			Expect(manager.PipelineSecretTemplate).To(Equal(conjur.DefaultPipelineSecretTemplate))
			Expect(manager.TeamSecretTemplate).To(Equal(conjur.DefaultTeamSecretTemplate))
		})

		It("passes on default parameters", func() {
			Expect(manager.Validate()).To(BeNil())
		})

		DescribeTable("passes if all Conjur credentials are specified",
			func(account, login, apiKey, tokenFile string) {
				manager.ConjurApplianceUrl = "http://conjur-test"
				manager.ConjurAccount = account
				manager.ConjurAuthnLogin = login
				manager.ConjurAuthnApiKey = apiKey
				manager.ConjurAuthnTokenFile = tokenFile
				Expect(manager.Validate()).To(BeNil())
			},
			Entry("account & login & apiKey", "account", "login", "apiKey", ""),
			Entry("account & login & tokenFile", "account", "login", "", "tokenFile"),
		)

		DescribeTable("fails on partial Conjur credentials",
			func(account, login, apiKey, tokenFile string) {
				manager.ConjurApplianceUrl = "http://conjur-test"
				manager.ConjurAccount = account
				manager.ConjurAuthnLogin = login
				manager.ConjurAuthnApiKey = apiKey
				manager.ConjurAuthnTokenFile = tokenFile
				Expect(manager.Validate()).ToNot(BeNil())
			},
			Entry("only account", "account", "", "", ""),
			Entry("only login", "", "login", "", ""),
			Entry("only apiKey", "", "", "apiKey", ""),
			Entry("only token file", "", "", "", "tokenFile"),
			Entry("account & login", "account", "login", "", ""),
			Entry("account & apiKey", "account", "", "apiKey", ""),
			Entry("account & tokenFile", "account", "", "", "tokenFile"),
			Entry("login & apiKey", "", "login", "apiKey", ""),
			Entry("login & tokenFile", "", "login", "", "tokenFile"),
			Entry("account & login & apiKey & tokenFile", "account", "login", "apikey", "tokenFile"),
		)

		It("passes on pipe secret template containing less specialization", func() {
			manager.PipelineSecretTemplate = "{{.Secret}}"
			Expect(manager.Validate()).To(BeNil())
		})

		It("passes on pipe secret template containing no specialization", func() {
			manager.PipelineSecretTemplate = "var"
			Expect(manager.Validate()).To(BeNil())
		})

		It("fails on empty pipe secret template", func() {
			manager.PipelineSecretTemplate = ""
			Expect(manager.Validate()).ToNot(BeNil())
		})

		It("fails on pipe secret template containing invalid parameters", func() {
			manager.PipelineSecretTemplate = "{{.Teams}}"
			Expect(manager.Validate()).ToNot(BeNil())
		})

		It("passes on team secret template containing less specialization", func() {
			manager.TeamSecretTemplate = "{{.Secret}}"
			Expect(manager.Validate()).To(BeNil())
		})

		It("passes on team secret template containing no specialization", func() {
			manager.TeamSecretTemplate = "var"
			Expect(manager.Validate()).To(BeNil())
		})

		It("fails on empty team secret template", func() {
			manager.TeamSecretTemplate = ""
			Expect(manager.Validate()).ToNot(BeNil())
		})

		It("fails on team secret template containing invalid parameters", func() {
			manager.TeamSecretTemplate = "{{.Teams}}"
			Expect(manager.Validate()).ToNot(BeNil())
		})
	})
})
