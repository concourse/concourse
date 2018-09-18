package ssm_test

import (
	"github.com/concourse/atc/creds/ssm"
	flags "github.com/jessevdk/go-flags"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("SsmManager", func() {
	var manager ssm.SsmManager

	Describe("IsConfigured()", func() {
		JustBeforeEach(func() {
			_, err := flags.ParseArgs(&manager, []string{})
			Expect(err).To(BeNil())
		})

		It("failes on empty SsmManager", func() {
			Expect(manager.IsConfigured()).To(BeFalse())
		})

		It("passes if AwsRegion is set", func() {
			manager.AwsRegion = "test-region"
			Expect(manager.IsConfigured()).To(BeTrue())
		})
	})

	Describe("Health()", func() {
	})

	Describe("Validate()", func() {
		JustBeforeEach(func() {
			manager = ssm.SsmManager{AwsRegion: "test-region"}
			_, err := flags.ParseArgs(&manager, []string{})
			Expect(err).To(BeNil())
			Expect(manager.PipelineSecretTemplate).To(Equal(ssm.DefaultPipelineSecretTemplate))
			Expect(manager.TeamSecretTemplate).To(Equal(ssm.DefaultTeamSecretTemplate))
		})

		It("passes on default parameters", func() {
			Expect(manager.Validate()).To(BeNil())
		})

		DescribeTable("passes if all aws credentials are specified",
			func(accessKey, secretKey, sessionToken string) {
				manager.AwsAccessKeyID = accessKey
				manager.AwsSecretAccessKey = secretKey
				manager.AwsSessionToken = sessionToken
				Expect(manager.Validate()).To(BeNil())
			},
			Entry("all values", "access", "secret", "token"),
			Entry("access & secret", "access", "secret", ""),
		)

		DescribeTable("fails on partial AWS credentials",
			func(accessKey, secretKey, sessionToken string) {
				manager.AwsAccessKeyID = accessKey
				manager.AwsSecretAccessKey = secretKey
				manager.AwsSessionToken = sessionToken
				Expect(manager.Validate()).ToNot(BeNil())
			},
			Entry("only access", "access", "", ""),
			Entry("access & token", "access", "", "token"),
			Entry("only secret", "", "secret", ""),
			Entry("secret & token", "", "secret", "token"),
			Entry("only token", "", "", "token"),
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
