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

		It("uses path-style default templates, with shared secrets under their own root", func() {
			Expect(manager.PipelineSecretTemplate).To(Equal("/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"))
			Expect(manager.TeamSecretTemplate).To(Equal("/concourse/{{.Team}}/{{.Secret}}"))
			Expect(manager.SharedSecretTemplate).To(Equal("/concourse-shared/{{.Secret}}"))
		})

		DescribeTable("rejects a template that cannot produce a legal secret ID",
			func(template string) {
				manager.SharedSecretTemplate = template
				Expect(manager.Validate()).To(MatchError(ContainSubstring("invalid Google Secret Manager secret ID")))
			},
			Entry("dotted", "/concourse.ci/{{.Secret}}"),
			Entry("space", "/concourse ci/{{.Secret}}"),
			Entry("non-ASCII letter", "/concoursé/{{.Secret}}"),
		)

		It("rejects an unparseable template", func() {
			manager.TeamSecretTemplate = "{{.Team"
			Expect(manager.Validate()).To(HaveOccurred())
		})

		It("accepts custom templates that give every placeholder its own segment", func() {
			manager.PipelineSecretTemplate = "/ci_creds/{{.Team}}/{{.Pipeline}}/{{.Secret}}"
			manager.TeamSecretTemplate = "ci_creds/{{.Team}}/{{.Secret}}"
			manager.SharedSecretTemplate = "/ci_shared/{{.Secret}}"
			Expect(manager.Validate()).To(BeNil())
		})

		DescribeTable("rejects a template whose segments would be ambiguous",
			func(template, message string) {
				manager.TeamSecretTemplate = template
				Expect(manager.Validate()).To(MatchError(ContainSubstring(message)))
			},
			Entry("old double-hyphen style", "concourse--{{.Team}}--{{.Secret}}", "must fill a whole segment"),
			Entry("placeholder sharing a segment", "/concourse/team-{{.Team}}/{{.Secret}}", "must fill a whole segment"),
			Entry("adjacent placeholders", "/concourse/{{.Team}}{{.Secret}}", "must fill a whole segment"),
			Entry("literal containing the ID separator", "/con--course/{{.Team}}/{{.Secret}}", "literal segment"),
			Entry("literal ending in a hyphen", "/concourse-/{{.Team}}/{{.Secret}}", "literal segment"),
			Entry("literal starting with a hyphen", "/-concourse/{{.Team}}/{{.Secret}}", "literal segment"),
			Entry("empty segment", "/concourse//{{.Team}}/{{.Secret}}", "literal segment"),
			Entry("trailing slash", "/concourse/{{.Team}}/{{.Secret}}/", "literal segment"),
			Entry("missing secret", "/concourse/{{.Team}}", "must contain {{.Secret}}"),
			Entry("repeated placeholder", "/concourse/{{.Team}}/{{.Team}}/{{.Secret}}", "at most once"),
			Entry("pipeline without team", "/concourse/{{.Pipeline}}/{{.Secret}}", "requires {{.Team}}"),
			Entry("secret only", "{{.Secret}}", "must begin with a literal segment"),
			Entry("team as the root", "/{{.Team}}/concourse/{{.Secret}}", "must begin with a literal segment"),
			Entry("conditional", "/concourse/{{if .Team}}{{.Team}}{{end}}/{{.Secret}}", "unsupported template construct"),
			Entry("function call", `/concourse/{{printf "%s" .Team}}/{{.Secret}}`, "unsupported template action"),
		)

		DescribeTable("rejects templates that can produce the same secret ID",
			func(pipeline, team, shared string) {
				manager.PipelineSecretTemplate = pipeline
				manager.TeamSecretTemplate = team
				manager.SharedSecretTemplate = shared
				Expect(manager.Validate()).To(MatchError(ContainSubstring("can produce the same secret ID")))
			},
			Entry("shared nested under the team root",
				gcpsecretmanager.DefaultPipelineSecretTemplate,
				gcpsecretmanager.DefaultTeamSecretTemplate,
				"/concourse/shared/{{.Secret}}"),
			Entry("team literal matching a pipeline placeholder",
				gcpsecretmanager.DefaultPipelineSecretTemplate,
				"/concourse/{{.Team}}/team/{{.Secret}}",
				gcpsecretmanager.DefaultSharedSecretTemplate),
			Entry("identical templates",
				gcpsecretmanager.DefaultPipelineSecretTemplate,
				gcpsecretmanager.DefaultTeamSecretTemplate,
				gcpsecretmanager.DefaultTeamSecretTemplate),
			Entry("identical apart from the leading slash",
				gcpsecretmanager.DefaultPipelineSecretTemplate,
				gcpsecretmanager.DefaultTeamSecretTemplate,
				"concourse/{{.Team}}/{{.Secret}}"),
		)

		DescribeTable("rejects a shared template that shares a root with the team or pipeline template",
			func(pipeline, team, shared string) {
				manager.PipelineSecretTemplate = pipeline
				manager.TeamSecretTemplate = team
				manager.SharedSecretTemplate = shared
				Expect(manager.Validate()).To(MatchError(ContainSubstring("own root")))
			},
			Entry("AWS-style shared template",
				gcpsecretmanager.DefaultPipelineSecretTemplate,
				gcpsecretmanager.DefaultTeamSecretTemplate,
				"/concourse/{{.Secret}}"),
			Entry("shared root matching only the pipeline template",
				"/ci-shared/{{.Team}}/{{.Pipeline}}/{{.Secret}}",
				gcpsecretmanager.DefaultTeamSecretTemplate,
				"/ci-shared/{{.Secret}}"),
		)

		It("accepts team and pipeline templates under different roots", func() {
			manager.PipelineSecretTemplate = "/ci-pipelines/{{.Team}}/{{.Pipeline}}/{{.Secret}}"
			manager.TeamSecretTemplate = "/ci-teams/{{.Team}}/{{.Secret}}"
			manager.SharedSecretTemplate = "/ci-shared/{{.Secret}}"
			Expect(manager.Validate()).To(BeNil())
		})
	})

	Describe("MarshalJSON()", func() {
		BeforeEach(func() {
			manager = gcpsecretmanager.Manager{
				ProjectID:              "my-test-project",
				CredentialsJSON:        `{"type":"service_account","private_key":"SUPER-SECRET-KEY"}`,
				CredentialsFile:        "/etc/concourse/sa-key.json",
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

		It("rejects ambiguous templates even when Validate was skipped", func() {
			manager = gcpsecretmanager.Manager{
				ProjectID:              "my-test-project",
				PipelineSecretTemplate: gcpsecretmanager.DefaultPipelineSecretTemplate,
				TeamSecretTemplate:     gcpsecretmanager.DefaultTeamSecretTemplate,
				SharedSecretTemplate:   "/concourse/shared/{{.Secret}}",
				SecretManager:          gcpsecretmanager.NewSecretManager(nil, nil, "my-test-project", 0, nil),
			}
			_, err := manager.NewSecretsFactory(nil)
			Expect(err).To(MatchError(ContainSubstring("can produce the same secret ID")))
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
			m, err := factory.NewInstance(map[string]any{
				"project":          "my-test-project",
				"credentials_json": `{"type":"service_account"}`,
			})
			Expect(err).ToNot(HaveOccurred())

			gcpManager, ok := m.(*gcpsecretmanager.Manager)
			Expect(ok).To(BeTrue())
			Expect(gcpManager.ProjectID).To(Equal("my-test-project"))
			Expect(gcpManager.RequestTimeout).To(Equal(gcpsecretmanager.DefaultRequestTimeout))
			Expect(gcpManager.SharedSecretTemplate).To(Equal(gcpsecretmanager.DefaultSharedSecretTemplate))
			Expect(m.Validate()).To(BeNil())
		})

		It("decodes a duration string for request_timeout", func() {
			m, err := factory.NewInstance(map[string]any{
				"project":          "my-test-project",
				"credentials_json": `{"type":"service_account"}`,
				"request_timeout":  "30s",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(m.(*gcpsecretmanager.Manager).RequestTimeout).To(Equal(30 * time.Second))
		})

		It("rejects credentials_file, so pipeline authors cannot pick a file on the web node", func() {
			_, err := factory.NewInstance(map[string]any{
				"project":          "my-test-project",
				"credentials_file": "/etc/passwd",
			})
			Expect(err).To(MatchError(ContainSubstring("credentials_file is not supported in a var_source")))
		})

		// 2026-10-09: pipeline authors control var_source configs, including
		// the project and templates. Without credentials of its own, a
		// var_source would borrow the web node's identity and could read any
		// team's secrets.
		It("rejects a var_source with no credentials, so it cannot borrow the web node's identity", func() {
			_, err := factory.NewInstance(map[string]any{
				"project":                "my-test-project",
				"shared_secret_template": "/concourse-shared/{{.Secret}}",
			})
			Expect(err).To(MatchError(ContainSubstring("credentials_json is required in a var_source")))
		})

		It("accepts credentials_json", func() {
			m, err := factory.NewInstance(map[string]any{
				"project":          "my-test-project",
				"credentials_json": `{"type":"service_account"}`,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(m.(*gcpsecretmanager.Manager).CredentialsJSON).To(Equal(`{"type":"service_account"}`))
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
