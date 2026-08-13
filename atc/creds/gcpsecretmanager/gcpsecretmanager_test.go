package gcpsecretmanager_test

import (
	"context"
	"errors"
	"hash/crc32"
	"time"

	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/concourse/concourse/atc/creds"
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
			"latest",
			time.Second,
			templates,
		)
	})

	Describe("NewSecretLookupPaths()", func() {
		It("builds double-hyphen delimited paths for a team and pipeline", func() {
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
				"concourse--mysecret",
			}))
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
		It("addresses the configured project and version", func() {
			_, _, found, err := secrets.Get("concourse--main--mysecret")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(lastCalled).To(Equal("projects/my-test-project/secrets/concourse--main--mysecret/versions/latest"))
		})

		It("honours a pinned numeric version", func() {
			secrets = NewSecretManager(lagertest.NewTestLogger("t"), api, testProject, "7", time.Second, templates)

			_, _, _, err := secrets.Get("concourse--main--mysecret")
			Expect(err).ToNot(HaveOccurred())
			Expect(lastCalled).To(Equal("projects/my-test-project/secrets/concourse--main--mysecret/versions/7"))
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
			secrets = NewSecretManager(lagertest.NewTestLogger("t"), api, testProject, "latest", 0, templates)

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

			It("rejects an ID longer than 255 characters", func() {
				long := ""
				for range 256 {
					long += "a"
				}

				_, _, _, err := secrets.Get(long)
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
