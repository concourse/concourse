package gcpsecretmanager_test

import (
	"context"
	"errors"
	"hash/crc32"
	"strings"
	"time"

	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/vars"
	gax "github.com/googleapis/gax-go/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	. "github.com/concourse/concourse/atc/creds/gcpsecretmanager"
	"github.com/concourse/concourse/atc/creds/gcpsecretmanager/gcpsecretmanagerfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const testProject = "my-test-project"

func crc32cOf(data []byte) *int64 {
	sum := int64(crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli)))
	return &sum
}

func payloadResponse(data string, withChecksum bool) *secretmanagerpb.AccessSecretVersionResponse {
	payload := &secretmanagerpb.SecretPayload{Data: []byte(data)}
	if withChecksum {
		payload.DataCrc32C = crc32cOf([]byte(data))
	}
	return &secretmanagerpb.AccessSecretVersionResponse{Payload: payload}
}

var _ = Describe("SecretManager", func() {
	var (
		api        *gcpsecretmanagerfakes.FakeSecretManagerAPI
		secrets    *SecretManager
		templates  []*creds.SecretTemplate
		lastCalled string
	)

	BeforeEach(func() {
		api = new(gcpsecretmanagerfakes.FakeSecretManagerAPI)
		lastCalled = ""

		api.AccessSecretVersionStub = func(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			lastCalled = req.GetName()
			return payloadResponse("hunter2", true), nil
		}

		pipelineTemplate, err := creds.BuildSecretTemplate("pipeline", DefaultPipelineSecretTemplate)
		Expect(err).ToNot(HaveOccurred())
		teamTemplate, err := creds.BuildSecretTemplate("team", DefaultTeamSecretTemplate)
		Expect(err).ToNot(HaveOccurred())
		sharedTemplate, err := creds.BuildSecretTemplate("shared", DefaultSharedSecretTemplate)
		Expect(err).ToNot(HaveOccurred())
		templates = []*creds.SecretTemplate{pipelineTemplate, teamTemplate, sharedTemplate}

		secrets = NewSecretManager(
			lagertest.NewTestLogger("gcpsecretmanager"),
			api,
			testProject,
			time.Second,
			templates,
		)
	})

	Describe("NewSecretLookupPaths()", func() {
		It("maps the conventional secret paths to secret IDs", func() {
			paths := secrets.NewSecretLookupPaths("main", "mypipeline", false)
			Expect(paths).To(HaveLen(3))

			var rendered []string
			for _, p := range paths {
				value, err := p.VariableToSecretPath("mysecret")
				Expect(err).ToNot(HaveOccurred())
				rendered = append(rendered, value)
			}

			Expect(rendered).To(Equal([]string{
				"concourse--main--mypipeline--mysecret",
				"concourse--main--mysecret",
				"concourse-shared--mysecret",
			}))
		})

		It("accepts names with single hyphens and underscores", func() {
			paths := secrets.NewSecretLookupPaths("team_a", "my-pipeline", false)

			value, err := paths[0].VariableToSecretPath("my_secret-1")
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal("concourse--team_a--my-pipeline--my_secret-1"))
		})

		It("maps a template with or without a leading slash to the same ID", func() {
			withSlash, err := creds.BuildSecretTemplate("with", "/ci/{{.Team}}/{{.Secret}}")
			Expect(err).ToNot(HaveOccurred())
			withoutSlash, err := creds.BuildSecretTemplate("without", "ci/{{.Team}}/{{.Secret}}")
			Expect(err).ToNot(HaveOccurred())

			secrets = NewSecretManager(lagertest.NewTestLogger("t"), api, testProject, 0, []*creds.SecretTemplate{withSlash, withoutSlash})

			for _, p := range secrets.NewSecretLookupPaths("main", "", false) {
				value, err := p.VariableToSecretPath("mysecret")
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(Equal("ci--main--mysecret"))
			}
		})

		DescribeTable("rejects a var name that could reach another scope or is not representable, on every path",
			func(varName string) {
				paths := secrets.NewSecretLookupPaths("main", "mypipeline", false)
				Expect(paths).To(HaveLen(3))

				for _, p := range paths {
					_, err := p.VariableToSecretPath(varName)
					Expect(err).To(MatchError(ErrInvalidSegment))
					Expect(err).To(MatchError(ContainSubstring("var name")))
				}
			},
			Entry("another team's secret through the shared path", "other-team/secret"),
			Entry("another pipeline's secret through the team path", "other-pipeline/secret"),
			Entry("the ID separator", "other-team--secret"),
			Entry("leading hyphen", "-secret"),
			Entry("trailing hyphen", "secret-"),
			Entry("only the separator", "--"),
			Entry("quoted dot", "my.secret"),
			Entry("at sign", "user@host"),
			Entry("non-ASCII letter", "sécret"),
		)

		DescribeTable("skips a scope whose team or pipeline name cannot be mapped, but still resolves the others",
			func(team, pipeline string, expected []string) {
				var rendered []string
				for _, p := range secrets.NewSecretLookupPaths(team, pipeline, false) {
					value, err := p.VariableToSecretPath("mysecret")
					Expect(err).ToNot(HaveOccurred())
					rendered = append(rendered, value)
				}

				Expect(rendered).To(Equal(expected))
			},
			Entry("pipeline containing a dot", "main", "release-1.2", []string{"concourse--main--mysecret", "concourse-shared--mysecret"}),
			Entry("pipeline containing a slash", "main", "a/b", []string{"concourse--main--mysecret", "concourse-shared--mysecret"}),
			Entry("pipeline containing the ID separator", "main", "a--b", []string{"concourse--main--mysecret", "concourse-shared--mysecret"}),
			Entry("pipeline with a leading hyphen", "main", "-pipeline", []string{"concourse--main--mysecret", "concourse-shared--mysecret"}),
			Entry("team containing a slash", "a/b", "mypipeline", []string{"concourse-shared--mysecret"}),
			Entry("team containing the ID separator", "a--b", "mypipeline", []string{"concourse-shared--mysecret"}),
			Entry("team with a trailing hyphen", "main-", "mypipeline", []string{"concourse-shared--mysecret"}),
			Entry("team with a non-ASCII letter", "équipe", "", []string{"concourse-shared--mysecret"}),
			Entry("no team", "", "", []string{"concourse-shared--mysecret"}),
		)

		// 2026-10-09: with no lookup paths, creds looks the raw var name up as
		// a secret ID, which would bypass every scope.
		It("fails every lookup, rather than returning no paths, when every scope is skipped", func() {
			sharedPerTeam, err := creds.BuildSecretTemplate("shared", "/concourse-shared/{{.Team}}/{{.Secret}}")
			Expect(err).ToNot(HaveOccurred())
			secrets = NewSecretManager(lagertest.NewTestLogger("t"), api, testProject, 0, []*creds.SecretTemplate{templates[0], templates[1], sharedPerTeam})

			variables := creds.NewVariables(secrets, creds.SecretLookupParams{Team: "a--b", Pipeline: "p"}, false)
			_, found, err := variables.Get(vars.Reference{Path: "concourse--victim--db-password"})

			Expect(err).To(MatchError(ErrInvalidSegment))
			Expect(found).To(BeFalse())
			Expect(api.AccessSecretVersionCallCount()).To(Equal(0))
		})

		// 2026-10-09: Concourse has no per-secret access control. Isolation
		// rests on every (scope, team, pipeline, var) mapping to its own ID.
		It("never maps two different lookups to the same secret ID", func() {
			names := []string{"a", "b", "a-b", "a_b", "a--b", "a/b", "-a", "a-", "a.b", "shared", ""}

			type owner struct{ scope, team, pipeline, secret string }
			owners := map[string]owner{}
			scopes := []string{"pipeline", "team", "shared"}

			for i, scope := range scopes {
				scoped := NewSecretManager(lagertest.NewTestLogger("t"), api, testProject, 0, templates[i:i+1])

				for _, team := range names {
					for _, pipeline := range names {
						for _, p := range scoped.NewSecretLookupPaths(team, pipeline, false) {
							for _, secret := range names {
								id, err := p.VariableToSecretPath(secret)
								if err != nil {
									continue
								}

								o := owner{scope: scope, secret: secret}
								if scope != "shared" {
									o.team = team
								}
								if scope == "pipeline" {
									o.pipeline = pipeline
								}

								if existing, seen := owners[id]; seen {
									Expect(existing).To(Equal(o), "secret ID %q", id)
								}
								owners[id] = o
							}
						}
					}
				}
			}

			Expect(owners).To(HaveKey("concourse--a-b--a_b--a"))
			Expect(owners).To(HaveKey("concourse--a_b--a-b"))
			Expect(owners).To(HaveKey("concourse-shared--shared"))
		})

		It("omits the pipeline-dependent path when there is no pipeline", func() {
			paths := secrets.NewSecretLookupPaths("main", "", false)
			Expect(paths).To(HaveLen(2))

			value, err := paths[0].VariableToSecretPath("mysecret")
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal("concourse--main--mysecret"))
		})
	})

	Describe("Get()", func() {
		It("addresses the configured project and always reads the latest version", func() {
			_, _, found, err := secrets.Get("concourse--main--mysecret")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(lastCalled).To(Equal("projects/my-test-project/secrets/concourse--main--mysecret/versions/latest"))
		})

		It("returns a plain payload as a string", func() {
			value, expiration, found, err := secrets.Get("concourse--main--mysecret")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(value).To(Equal("hunter2"))
			Expect(expiration).To(BeNil())
		})

		It("returns a JSON object payload as a map so ((secret.field)) resolves", func() {
			api.AccessSecretVersionReturns(payloadResponse(`{"user":"admin","pass":"s3cret"}`, true), nil)

			value, _, found, err := secrets.Get("concourse--main--mysecret")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(value).To(Equal(map[string]any{"user": "admin", "pass": "s3cret"}))
		})

		It("does not coerce a JSON scalar payload into a map", func() {
			api.AccessSecretVersionReturns(payloadResponse(`"just-a-string"`, true), nil)

			value, _, _, err := secrets.Get("concourse--main--mysecret")
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal(`"just-a-string"`))
		})

		It("does not turn a literal JSON null into a nil map", func() {
			api.AccessSecretVersionReturns(payloadResponse("null", true), nil)

			value, _, found, err := secrets.Get("concourse--main--mysecret")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(value).To(Equal("null"))
		})

		It("falls back to the default timeout when none is configured", func() {
			secrets = NewSecretManager(lagertest.NewTestLogger("t"), api, testProject, 0, templates)

			_, _, found, err := secrets.Get("concourse--main--mysecret")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		It("accepts a payload with no checksum", func() {
			api.AccessSecretVersionReturns(payloadResponse("hunter2", false), nil)

			value, _, found, err := secrets.Get("concourse--main--mysecret")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(value).To(Equal("hunter2"))
		})

		Context("when the secret is absent", func() {
			It("reports not-found without an error for NotFound", func() {
				api.AccessSecretVersionReturns(nil, status.Error(codes.NotFound, "nope"))

				value, _, found, err := secrets.Get("concourse--main--mysecret")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(value).To(BeNil())
			})

			It("reports not-found without an error for FailedPrecondition", func() {
				api.AccessSecretVersionReturns(nil, status.Error(codes.FailedPrecondition, "version destroyed"))

				_, _, found, err := secrets.Get("concourse--main--mysecret")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("fail closed", func() {
			It("propagates PermissionDenied rather than masking it as not-found", func() {
				api.AccessSecretVersionReturns(nil, status.Error(codes.PermissionDenied, "denied"))

				_, _, found, err := secrets.Get("concourse--main--mysecret")
				Expect(err).To(HaveOccurred())
				Expect(found).To(BeFalse())
			})

			It("propagates a transport error", func() {
				api.AccessSecretVersionReturns(nil, errors.New("connection reset"))

				_, _, found, err := secrets.Get("concourse--main--mysecret")
				Expect(err).To(MatchError(ContainSubstring("connection reset")))
				Expect(found).To(BeFalse())
			})

			It("errors when the response carries no payload", func() {
				api.AccessSecretVersionReturns(&secretmanagerpb.AccessSecretVersionResponse{}, nil)

				_, _, found, err := secrets.Get("concourse--main--mysecret")
				Expect(err).To(MatchError(ContainSubstring("no payload")))
				Expect(found).To(BeFalse())
			})

			It("rejects a payload whose crc32c does not match", func() {
				corrupt := payloadResponse("hunter2", true)
				corrupt.Payload.Data = []byte("tampered")
				api.AccessSecretVersionReturns(corrupt, nil)

				value, _, found, err := secrets.Get("concourse--main--mysecret")
				Expect(err).To(MatchError(ContainSubstring("crc32c checksum mismatch")))
				Expect(found).To(BeFalse())
				Expect(value).To(BeNil())
			})
		})

		Context("secret ID validation", func() {
			DescribeTable("rejects an ID that is not addressable, without calling the API",
				func(secretID string) {
					_, _, found, err := secrets.Get(secretID)

					Expect(err).To(MatchError(ErrInvalidSecretID))
					Expect(found).To(BeFalse())
					Expect(api.AccessSecretVersionCallCount()).To(Equal(0))
				},
				Entry("path traversal to another project", "../../other-project/secrets/admin"),
				Entry("embedded resource path", "foo/versions/1"),
				Entry("trailing slash", "mysecret/"),
				Entry("empty", ""),
				Entry("slash only", "/"),
				Entry("dot segment", ".."),
				Entry("space", "my secret"),
				Entry("wildcard", "*"),
				Entry("newline", "mysecret\nfoo"),
				Entry("percent encoding", "mysecret%2Fadmin"),
			)

			It("reports an ID longer than 255 characters as not found, without calling the API", func() {
				_, _, found, err := secrets.Get(strings.Repeat("a", 256))
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(api.AccessSecretVersionCallCount()).To(Equal(0))
			})

			It("still rejects an over-long ID containing illegal characters", func() {
				_, _, _, err := secrets.Get(strings.Repeat("a/", 200))
				Expect(err).To(MatchError(ErrInvalidSecretID))
				Expect(api.AccessSecretVersionCallCount()).To(Equal(0))
			})

			It("accepts letters, numerals, hyphens and underscores", func() {
				_, _, found, err := secrets.Get("Concourse--main_1--my-secret_2")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})
	})
})
