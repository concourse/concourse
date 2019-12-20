package vault_test

import (
	"github.com/concourse/concourse/atc/creds/vault"
	"github.com/jessevdk/go-flags"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("VaultManager", func() {
	var manager vault.VaultManager

	Describe("IsConfigured()", func() {
		JustBeforeEach(func() {
			_, err := flags.ParseArgs(&manager, []string{})
			Expect(err).To(BeNil())
		})

		It("fails on empty Manager", func() {
			Expect(manager.IsConfigured()).To(BeFalse())
		})

		It("passes if URL is set", func() {
			manager.URL = "http://vault"
			Expect(manager.IsConfigured()).To(BeTrue())
		})
	})

	Describe("Validate()", func() {
		JustBeforeEach(func() {
			manager = vault.VaultManager{URL: "http://vault", Auth: vault.AuthConfig{ClientToken: "xxx"}}
			_, err := flags.ParseArgs(&manager, []string{})
			Expect(err).To(BeNil())
			Expect(manager.SharedPath).To(Equal(""))
			Expect(manager.PathPrefix).To(Equal("/concourse"))
			Expect(manager.Namespace).To(Equal(""))
		})

		It("passes on default parameters", func() {
			Expect(manager.Validate()).To(BeNil())
		})

		DescribeTable("passes if all vault credentials are specified",
			func(backend, clientToken string) {
				manager.Auth.Backend = backend
				manager.Auth.ClientToken = clientToken
				Expect(manager.Validate()).To(BeNil())
			},
			Entry("all values", "backend", "clientToken"),
			Entry("only clientToken", "", "clientToken"),
		)

		It("fails on missing vault auth credentials", func() {
			manager.Auth = vault.AuthConfig{}
			Expect(manager.Validate()).ToNot(BeNil())
		})
	})
})
