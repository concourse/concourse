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
			Expect(manager.SegmentDelimiter).To(Equal(gcpsecretmanager.DefaultSegmentDelimiter))
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

		It("accepts custom templates that delimit every placeholder", func() {
			manager.PipelineSecretTemplate = "ci_creds--{{.Team}}--{{.Pipeline}}--{{.Secret}}"
			manager.TeamSecretTemplate = "ci_creds--{{.Team}}--{{.Secret}}"
			manager.SharedSecretTemplate = "{{.Secret}}"
			Expect(manager.Validate()).To(BeNil())
		})

		DescribeTable("rejects a template whose segments would be ambiguous",
			func(template, message string) {
				manager.TeamSecretTemplate = template
				Expect(manager.Validate()).To(MatchError(ContainSubstring(message)))
			},
			Entry("underscore delimiter", "concourse__{{.Team}}__{{.Secret}}", "separated from the rest of the template"),
			Entry("single hyphen delimiter", "concourse-{{.Team}}-{{.Secret}}", "separated from the rest of the template"),
			Entry("adjacent placeholders", "concourse--{{.Team}}{{.Secret}}", "separated from the rest of the template"),
			Entry("triple hyphen", "concourse---{{.Team}}--{{.Secret}}", "separated from the rest of the template"),
			Entry("literal ending in a hyphen", "concourse---x--{{.Team}}--{{.Secret}}", "must not begin or end with a character of the delimiter"),
			Entry("leading delimiter", "--{{.Team}}--{{.Secret}}", "must be non-empty"),
			Entry("empty segment", "concourse----{{.Team}}--{{.Secret}}", "must be non-empty"),
			Entry("missing secret", "concourse--{{.Team}}", "must contain {{.Secret}}"),
			Entry("repeated placeholder", "concourse--{{.Team}}--{{.Team}}--{{.Secret}}", "at most once"),
			Entry("pipeline without team", "concourse--{{.Pipeline}}--{{.Secret}}", "requires {{.Team}}"),
			Entry("conditional", "concourse--{{if .Team}}{{.Team}}{{end}}--{{.Secret}}", "unsupported template construct"),
			Entry("function call", `concourse--{{printf "%s" .Team}}--{{.Secret}}`, "unsupported template action"),
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
				"concourse--shared--{{.Secret}}"),
			Entry("team literal matching a pipeline placeholder",
				gcpsecretmanager.DefaultPipelineSecretTemplate,
				"concourse--{{.Team}}--team--{{.Secret}}",
				gcpsecretmanager.DefaultSharedSecretTemplate),
			Entry("identical templates",
				gcpsecretmanager.DefaultPipelineSecretTemplate,
				gcpsecretmanager.DefaultTeamSecretTemplate,
				gcpsecretmanager.DefaultTeamSecretTemplate),
		)

		Describe("segment delimiter", func() {
			It("accepts templates that use a custom delimiter", func() {
				manager.SegmentDelimiter = "__"
				manager.PipelineSecretTemplate = "ci__{{.Team}}__{{.Pipeline}}__{{.Secret}}"
				manager.TeamSecretTemplate = "ci__{{.Team}}__{{.Secret}}"
				manager.SharedSecretTemplate = "ci-shared__{{.Secret}}"
				Expect(manager.Validate()).To(BeNil())
			})

			It("accepts a longer delimiter", func() {
				manager.SegmentDelimiter = "---"
				manager.PipelineSecretTemplate = "concourse---{{.Team}}---{{.Pipeline}}---{{.Secret}}"
				manager.TeamSecretTemplate = "concourse---{{.Team}}---{{.Secret}}"
				manager.SharedSecretTemplate = "concourse-shared---{{.Secret}}"
				Expect(manager.Validate()).To(BeNil())
			})

			It("treats an empty delimiter as the default", func() {
				manager.SegmentDelimiter = ""
				Expect(manager.Validate()).To(BeNil())
			})

			It("rejects the default templates when the delimiter changes", func() {
				manager.SegmentDelimiter = "__"
				Expect(manager.Validate()).To(MatchError(ContainSubstring(`separated from the rest of the template by the delimiter "__"`)))
			})

			It("detects overlapping templates under a custom delimiter", func() {
				manager.SegmentDelimiter = "_"
				manager.PipelineSecretTemplate = "ci_{{.Team}}_{{.Pipeline}}_{{.Secret}}"
				manager.TeamSecretTemplate = "ci_{{.Team}}_{{.Secret}}"
				manager.SharedSecretTemplate = "ci_shared_{{.Secret}}"
				Expect(manager.Validate()).To(MatchError(ContainSubstring("can produce the same secret ID")))
			})

			DescribeTable("rejects an invalid delimiter",
				func(delimiter string) {
					manager.SegmentDelimiter = delimiter
					Expect(manager.Validate()).To(MatchError(ContainSubstring("invalid segment delimiter")))
				},
				Entry("slash", "/"),
				Entry("dot", "."),
				Entry("letter", "x"),
				Entry("mixed with a letter", "-x-"),
			)
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
			Expect(out).To(HaveKeyWithValue("segment_delimiter", gcpsecretmanager.DefaultSegmentDelimiter))
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
				SharedSecretTemplate:   "concourse--shared--{{.Secret}}",
				SecretManager:          gcpsecretmanager.NewSecretManager(nil, nil, "my-test-project", 0, nil, ""),
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
			m, err := factory.NewInstance(map[string]any{"project": "my-test-project"})
			Expect(err).ToNot(HaveOccurred())

			gcpManager, ok := m.(*gcpsecretmanager.Manager)
			Expect(ok).To(BeTrue())
			Expect(gcpManager.ProjectID).To(Equal("my-test-project"))
			Expect(gcpManager.RequestTimeout).To(Equal(gcpsecretmanager.DefaultRequestTimeout))
			Expect(gcpManager.SharedSecretTemplate).To(Equal(gcpsecretmanager.DefaultSharedSecretTemplate))
			Expect(gcpManager.SegmentDelimiter).To(Equal(gcpsecretmanager.DefaultSegmentDelimiter))
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
