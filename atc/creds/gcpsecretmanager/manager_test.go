package gcpsecretmanager_test

import (
	"encoding/json"
	"time"

	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/gcpsecretmanager"
	"github.com/jessevdk/go-flags"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manager", func() {
	var manager gcpsecretmanager.Manager

	Describe("IsConfigured()", func() {
		JustBeforeEach(func() {
			_, err := flags.ParseArgs(&manager, []string{})
			Expect(err).To(BeNil())
		})

		It("fails on empty Manager", func() {
			Expect(manager.IsConfigured()).To(BeFalse())
		})

		It("passes if ProjectID is set", func() {
			manager.ProjectID = "my-test-project"
			Expect(manager.IsConfigured()).To(BeTrue())
		})
	})

	Describe("Validate()", func() {
		BeforeEach(func() {
			manager = gcpsecretmanager.Manager{ProjectID: "my-test-project"}
			_, err := flags.ParseArgs(&manager, []string{})
			Expect(err).To(BeNil())

			Expect(manager.PipelineSecretTemplate).To(Equal(gcpsecretmanager.DefaultPipelineSecretTemplate))
			Expect(manager.TeamSecretTemplate).To(Equal(gcpsecretmanager.DefaultTeamSecretTemplate))
			Expect(manager.SharedSecretTemplate).To(Equal(gcpsecretmanager.DefaultSharedSecretTemplate))
		})

		It("passes on default parameters", func() {
			Expect(manager.Validate()).To(BeNil())
		})

		It("passes with a numeric project number", func() {
			manager.ProjectID = "123456789012"
			Expect(manager.Validate()).To(BeNil())
		})

		DescribeTable("rejects an invalid project",
			func(project string) {
				manager.ProjectID = project
				Expect(manager.Validate()).To(HaveOccurred())
			},
			Entry("empty", ""),
			Entry("path traversal", "../other-project"),
			Entry("uppercase", "My-Project"),
			Entry("too short", "abc"),
			Entry("trailing hyphen", "my-project-"),
			Entry("slash", "proj/ect"),
		)

		DescribeTable("accepts a valid secret version",
			func(version string) {
				manager.SecretVersion = version
				Expect(manager.Validate()).To(BeNil())
			},
			Entry("latest", "latest"),
			Entry("numeric", "42"),
		)

		DescribeTable("rejects an invalid secret version",
			func(version string) {
				manager.SecretVersion = version
				Expect(manager.Validate()).To(HaveOccurred())
			},
			Entry("path traversal", "latest/../../admin"),
			Entry("wildcard", "*"),
			Entry("arbitrary", "newest"),
		)

		It("rejects both credentials file and credentials json", func() {
			manager.CredentialsFile = "/tmp/key.json"
			manager.CredentialsJSON = `{"type":"service_account"}`
			Expect(manager.Validate()).To(MatchError(ContainSubstring("only one of")))
		})

		It("accepts credentials file alone", func() {
			manager.CredentialsFile = "/tmp/key.json"
			Expect(manager.Validate()).To(BeNil())
		})

		It("rejects a negative request timeout", func() {
			manager.RequestTimeout = -1 * time.Second
			Expect(manager.Validate()).To(HaveOccurred())
		})

		DescribeTable("rejects a template that cannot produce a legal secret ID",
			func(template string) {
				manager.SharedSecretTemplate = template
				Expect(manager.Validate()).To(MatchError(ContainSubstring("invalid Google Secret Manager secret ID")))
			},
			Entry("path style", "/concourse/{{.Secret}}"),
			Entry("slash delimiter", "concourse/{{.Secret}}"),
			Entry("dotted", "concourse.{{.Secret}}"),
		)

		It("rejects an unparseable template", func() {
			manager.TeamSecretTemplate = "{{.Team"
			Expect(manager.Validate()).To(HaveOccurred())
		})

		It("accepts a custom underscore-delimited template", func() {
			manager.SharedSecretTemplate = "concourse__{{.Secret}}"
			Expect(manager.Validate()).To(BeNil())
		})
	})

	Describe("MarshalJSON()", func() {
		BeforeEach(func() {
			manager = gcpsecretmanager.Manager{
				ProjectID:              "my-test-project",
				CredentialsJSON:        `{"type":"service_account","private_key":"SUPER-SECRET-KEY"}`,
				CredentialsFile:        "/etc/concourse/sa-key.json",
				SecretVersion:          "latest",
				PipelineSecretTemplate: gcpsecretmanager.DefaultPipelineSecretTemplate,
				TeamSecretTemplate:     gcpsecretmanager.DefaultTeamSecretTemplate,
				SharedSecretTemplate:   gcpsecretmanager.DefaultSharedSecretTemplate,
			}
		})

		It("never leaks credentials", func() {
			body, err := json.Marshal(&manager)
			Expect(err).ToNot(HaveOccurred())

			Expect(string(body)).ToNot(ContainSubstring("SUPER-SECRET-KEY"))
			Expect(string(body)).ToNot(ContainSubstring("private_key"))
			Expect(string(body)).ToNot(ContainSubstring("sa-key.json"))
			Expect(string(body)).ToNot(ContainSubstring("credentials"))
		})

		It("reports non-sensitive configuration", func() {
			body, err := json.Marshal(&manager)
			Expect(err).ToNot(HaveOccurred())

			var out map[string]any
			Expect(json.Unmarshal(body, &out)).To(Succeed())

			Expect(out).To(HaveKeyWithValue("project", "my-test-project"))
			Expect(out).To(HaveKeyWithValue("secret_version", "latest"))
			Expect(out).To(HaveKeyWithValue("shared_secret_template", gcpsecretmanager.DefaultSharedSecretTemplate))
			Expect(out).To(HaveKey("health"))
		})

		It("reports unhealthy rather than panicking when uninitialized", func() {
			body, err := json.Marshal(&manager)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(body)).To(ContainSubstring("not initialized"))
		})
	})

	Describe("NewSecretsFactory()", func() {
		It("fails when the manager has not been initialized", func() {
			manager = gcpsecretmanager.Manager{ProjectID: "my-test-project"}
			_, err := manager.NewSecretsFactory(nil)
			Expect(err).To(MatchError(ContainSubstring("not initialized")))
		})
	})

	Describe("registration", func() {
		It("registers itself as a credential manager", func() {
			Expect(creds.ManagerFactories()).To(HaveKey("gcpsecretmanager"))
		})
	})

	Describe("NewInstance()", func() {
		var factory creds.ManagerFactory

		BeforeEach(func() {
			factory = gcpsecretmanager.NewManagerFactory()
		})

		It("applies defaults for a minimal var_source config", func() {
			m, err := factory.NewInstance(map[string]any{"project": "my-test-project"})
			Expect(err).ToNot(HaveOccurred())

			gcpManager, ok := m.(*gcpsecretmanager.Manager)
			Expect(ok).To(BeTrue())
			Expect(gcpManager.ProjectID).To(Equal("my-test-project"))
			Expect(gcpManager.SecretVersion).To(Equal(gcpsecretmanager.DefaultSecretVersion))
			Expect(gcpManager.RequestTimeout).To(Equal(gcpsecretmanager.DefaultRequestTimeout))
			Expect(gcpManager.SharedSecretTemplate).To(Equal(gcpsecretmanager.DefaultSharedSecretTemplate))
			Expect(m.Validate()).To(BeNil())
		})

		It("decodes a duration string for request_timeout", func() {
			m, err := factory.NewInstance(map[string]any{
				"project":         "my-test-project",
				"request_timeout": "30s",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(m.(*gcpsecretmanager.Manager).RequestTimeout).To(Equal(30 * time.Second))
		})

		It("rejects unknown config keys", func() {
			_, err := factory.NewInstance(map[string]any{
				"project":  "my-test-project",
				"nonsense": "value",
			})
			Expect(err).To(HaveOccurred())
		})
	})
})
