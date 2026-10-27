package idtoken_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/atc/creds"
	"github.com/concourse/concourse/v8/atc/creds/idtoken"
	"github.com/concourse/concourse/v8/atc/db"
	"github.com/concourse/concourse/v8/atc/db/dbfakes"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	testIssuer     = "https://concourse.test"
	tokenExpiresIn = 15 * time.Minute
)

var _ = Describe("IDToken TokenGenerator", func() {
	var rsaSigningKey db.SigningKey
	var rsaVerificationKey jose.JSONWebKey
	var ecSigningKey db.SigningKey
	var ecVerificationKey jose.JSONWebKey
	var signingKeyFactory db.SigningKeyFactory
	var tokenGenerator *idtoken.TokenGenerator
	var params creds.SecretLookupParams

	BeforeEach(func() {
		rsaSigningKey = createFakeSigningKey(*rsaJWK, time.Now())
		ecSigningKey = createFakeSigningKey(*ecJWK, time.Now())

		rsaVerificationKey = rsaJWK.Public()
		ecVerificationKey = ecJWK.Public()

		signingKeyFactoryFake := &dbfakes.FakeSigningKeyFactory{}
		signingKeyFactoryFake.GetAllKeysReturns([]db.SigningKey{
			rsaSigningKey,
			ecSigningKey,
		}, nil)

		signingKeyFactoryFake.GetNewestKeyStub = func(skt db.SigningKeyType) (db.SigningKey, error) {
			switch skt {
			case db.SigningKeyTypeRSA:
				return rsaSigningKey, nil
			case db.SigningKeyTypeEC:
				return ecSigningKey, nil
			}
			return nil, fmt.Errorf("not found")
		}
		signingKeyFactory = signingKeyFactoryFake

		tokenGenerator = &idtoken.TokenGenerator{
			Issuer:            testIssuer,
			SigningKeyFactory: signingKeyFactory,
			ExpiresIn:         tokenExpiresIn,
		}

		params = creds.SecretLookupParams{
			Team:     "main",
			Pipeline: "idtoken",
			InstanceVars: atc.InstanceVars{
				"foo": "bar",
			},
			Job: "testjob",
		}
	})

	It("generates a valid token", func() {
		token, _, err := tokenGenerator.GenerateToken(params)
		Expect(err).ToNot(HaveOccurred())

		parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{idtoken.DefaultAlgorithm})
		Expect(err).ToNot(HaveOccurred())

		claims := jwt.Claims{}
		err = parsed.Claims(rsaVerificationKey, &claims)
		Expect(err).ToNot(HaveOccurred())
		Expect(claims.Subject).To(Equal(params.Team + "/" + params.Pipeline))
	})

	It("sets a unique jti on every token", func() {
		token, _, err := tokenGenerator.GenerateToken(params)
		Expect(err).ToNot(HaveOccurred())

		parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{idtoken.DefaultAlgorithm})
		Expect(err).ToNot(HaveOccurred())

		claims := jwt.Claims{}
		err = parsed.Claims(rsaVerificationKey, &claims)
		Expect(err).ToNot(HaveOccurred())
		Expect(claims.ID).ToNot(BeEmpty())

		otherToken, _, err := tokenGenerator.GenerateToken(params)
		Expect(err).ToNot(HaveOccurred())

		otherParsed, err := jwt.ParseSigned(otherToken, []jose.SignatureAlgorithm{idtoken.DefaultAlgorithm})
		Expect(err).ToNot(HaveOccurred())

		otherClaims := jwt.Claims{}
		err = otherParsed.Claims(rsaVerificationKey, &otherClaims)
		Expect(err).ToNot(HaveOccurred())
		Expect(otherClaims.ID).ToNot(BeEmpty())

		Expect(otherClaims.ID).ToNot(Equal(claims.ID))
	})

	It("respects subject scope team", func() {
		tokenGenerator.SubjectScope = idtoken.SubjectScopeTeam
		token, _, err := tokenGenerator.GenerateToken(params)
		Expect(err).ToNot(HaveOccurred())

		parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{idtoken.DefaultAlgorithm})
		Expect(err).ToNot(HaveOccurred())

		claims := jwt.Claims{}
		err = parsed.Claims(rsaVerificationKey, &claims)
		Expect(err).ToNot(HaveOccurred())

		Expect(claims.Subject).To(Equal(params.Team))
	})

	It("respects subject scope instance", func() {
		tokenGenerator.SubjectScope = idtoken.SubjectScopeInstance
		token, _, err := tokenGenerator.GenerateToken(params)
		Expect(err).ToNot(HaveOccurred())

		parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{idtoken.DefaultAlgorithm})
		Expect(err).ToNot(HaveOccurred())

		claims := jwt.Claims{}
		err = parsed.Claims(rsaVerificationKey, &claims)
		Expect(err).ToNot(HaveOccurred())

		Expect(claims.Subject).To(Equal(params.Team + "/" + params.Pipeline + "/" + params.InstanceVars.String()))
	})

	It("respects subject scope job", func() {
		tokenGenerator.SubjectScope = idtoken.SubjectScopeJob
		token, _, err := tokenGenerator.GenerateToken(params)
		Expect(err).ToNot(HaveOccurred())

		parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{idtoken.DefaultAlgorithm})
		Expect(err).ToNot(HaveOccurred())

		claims := jwt.Claims{}
		err = parsed.Claims(rsaVerificationKey, &claims)
		Expect(err).ToNot(HaveOccurred())

		Expect(claims.Subject).To(Equal(params.Team + "/" + params.Pipeline + "/" + params.InstanceVars.String() + "/" + params.Job))
	})

	It("escapes sub parts safely", func() {
		params = creds.SecretLookupParams{
			Team:     "fake/team",
			Pipeline: "fake/pipeline",
			InstanceVars: atc.InstanceVars{
				"fake/foo": "fake/bar",
			},
			Job: "fake/job",
		}
		tokenGenerator.SubjectScope = idtoken.SubjectScopeJob
		token, _, err := tokenGenerator.GenerateToken(params)
		Expect(err).ToNot(HaveOccurred())

		parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{idtoken.DefaultAlgorithm})
		Expect(err).ToNot(HaveOccurred())

		claims := jwt.Claims{}
		err = parsed.Claims(rsaVerificationKey, &claims)
		Expect(err).ToNot(HaveOccurred())

		Expect(claims.Subject).To(Equal("fake%2Fteam/fake%2Fpipeline/\"fake%2Ffoo\":\"fake%2Fbar\"/fake%2Fjob"))
	})

	It("adds aud claim when requested", func() {
		tokenGenerator.Audience = []string{"testaud"}
		token, _, err := tokenGenerator.GenerateToken(params)
		Expect(err).ToNot(HaveOccurred())

		parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{idtoken.DefaultAlgorithm})
		Expect(err).ToNot(HaveOccurred())

		claims := jwt.Claims{}
		err = parsed.Claims(rsaVerificationKey, &claims)
		Expect(err).ToNot(HaveOccurred())

		Expect(claims.Audience).To(ContainElement("testaud"))
	})

	It("uses ES256 when requested", func() {
		tokenGenerator.Algorithm = jose.ES256
		token, _, err := tokenGenerator.GenerateToken(params)
		Expect(err).ToNot(HaveOccurred())

		parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{tokenGenerator.Algorithm})
		Expect(err).ToNot(HaveOccurred())

		claims := jwt.Claims{}
		err = parsed.Claims(ecVerificationKey, &claims)
		Expect(err).ToNot(HaveOccurred())

		Expect(parsed.Headers[0].Algorithm).To(Equal("ES256"))
	})

	Context("AWS session tags", func() {
		type awsTagsClaims struct {
			Tags *struct {
				PrincipalTags map[string][]string `json:"principal_tags"`
			} `json:"https://aws.amazon.com/tags"`
		}

		parseTags := func(token string) awsTagsClaims {
			parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{idtoken.DefaultAlgorithm})
			Expect(err).ToNot(HaveOccurred())

			var claims awsTagsClaims
			err = parsed.Claims(rsaVerificationKey, &claims)
			Expect(err).ToNot(HaveOccurred())
			return claims
		}

		It("are not added by default", func() {
			token, _, err := tokenGenerator.GenerateToken(params)
			Expect(err).ToNot(HaveOccurred())

			Expect(parseTags(token).Tags).To(BeNil())
		})

		Context("when enabled", func() {
			BeforeEach(func() {
				tokenGenerator.AWSSessionTags = true
			})

			It("adds team, pipeline, job and instance vars as principal tags", func() {
				token, _, err := tokenGenerator.GenerateToken(params)
				Expect(err).ToNot(HaveOccurred())

				claims := parseTags(token)
				Expect(claims.Tags).ToNot(BeNil())
				Expect(claims.Tags.PrincipalTags).To(Equal(map[string][]string{
					"team":             {"main"},
					"pipeline":         {"idtoken"},
					"job":              {"testjob"},
					"instance_var:foo": {"bar"},
				}))
			})

			It("flattens nested and non-string instance vars", func() {
				params.InstanceVars = atc.InstanceVars{
					"env": "prod",
					"app": map[string]any{
						"name":     "api",
						"replicas": 3,
					},
					"canary": true,
				}
				token, _, err := tokenGenerator.GenerateToken(params)
				Expect(err).ToNot(HaveOccurred())

				Expect(parseTags(token).Tags.PrincipalTags).To(Equal(map[string][]string{
					"team":                      {"main"},
					"pipeline":                  {"idtoken"},
					"job":                       {"testjob"},
					"instance_var:env":          {"prod"},
					"instance_var:app.name":     {"api"},
					"instance_var:app.replicas": {"3"},
					"instance_var:canary":       {"true"},
				}))
			})

			It("omits instance vars when there are none", func() {
				params.InstanceVars = nil
				token, _, err := tokenGenerator.GenerateToken(params)
				Expect(err).ToNot(HaveOccurred())

				Expect(parseTags(token).Tags.PrincipalTags).To(HaveLen(3))
			})

			DescribeTable("rejects instance vars AWS does not accept",
				func(instanceVars atc.InstanceVars, errorMessage string) {
					params.InstanceVars = instanceVars
					_, _, err := tokenGenerator.GenerateToken(params)
					Expect(err).To(MatchError(ContainSubstring(errorMessage)))
				},
				Entry("illegal character in value", atc.InstanceVars{"env": "prod#1"}, "only letters"),
				Entry("illegal character in key", atc.InstanceVars{"env#": "prod"}, "only letters"),
				Entry("list value", atc.InstanceVars{"regions": []any{"a", "b"}}, "only letters"),
				Entry("key too long", atc.InstanceVars{strings.Repeat("a", 116): "x"}, "key longer than 128"),
				Entry("keys only differ in case", atc.InstanceVars{"env": "a", "Env": "b"}, "conflicting aws session tags"),
				Entry("nested key equals dotted key", atc.InstanceVars{"a.b": "x", "a": map[string]any{"b": "y"}}, "conflicting aws session tags"),
				Entry("too many tags", func() atc.InstanceVars {
					ivars := atc.InstanceVars{}
					for i := range 48 {
						ivars[fmt.Sprintf("var%d", i)] = "x"
					}
					return ivars
				}(), "more than 50"),
			)

			It("omits job when there is none", func() {
				params.Job = ""
				token, _, err := tokenGenerator.GenerateToken(params)
				Expect(err).ToNot(HaveOccurred())

				Expect(parseTags(token).Tags.PrincipalTags).ToNot(HaveKey("job"))
			})

			It("does not escape slashes in tag values", func() {
				params.Pipeline = "fake/pipeline"
				token, _, err := tokenGenerator.GenerateToken(params)
				Expect(err).ToNot(HaveOccurred())

				Expect(parseTags(token).Tags.PrincipalTags["pipeline"]).To(Equal([]string{"fake/pipeline"}))
			})

			DescribeTable("rejects values AWS does not accept",
				func(pipeline string) {
					params.Pipeline = pipeline
					_, _, err := tokenGenerator.GenerateToken(params)
					Expect(err).To(MatchError(ContainSubstring("aws session tag")))
				},
				Entry("illegal character", "my#pipeline"),
				Entry("reserved prefix", "aws:pipeline"),
				Entry("too long", strings.Repeat("a", 257)),
			)
		})
	})

	Context("Generated Token", func() {
		type claimStruct struct {
			jwt.Claims
			Team         string           `json:"team"`
			Pipeline     string           `json:"pipeline"`
			InstanceVars atc.InstanceVars `json:"instance_vars"`
			Job          string           `json:"job"`
		}

		var generatedToken string
		var claims claimStruct
		var generatedAt time.Time

		BeforeEach(func() {
			var err error
			generatedToken, _, err = tokenGenerator.GenerateToken(params)
			Expect(err).ToNot(HaveOccurred())

			generatedAt = time.Now()

			parsed, err := jwt.ParseSigned(generatedToken, []jose.SignatureAlgorithm{idtoken.DefaultAlgorithm})
			Expect(err).ToNot(HaveOccurred())

			err = parsed.Claims(rsaVerificationKey, &claims)
			Expect(err).ToNot(HaveOccurred())
		})

		It("contains the correct subject", func() {
			Expect(claims.Subject).To(Equal(params.Team + "/" + params.Pipeline))
		})

		It("contains the correct issuer", func() {
			Expect(claims.Issuer).To(Equal(testIssuer))
		})

		It("has the correct expiration time", func() {
			exp := claims.Expiry.Time()
			expected := generatedAt.Add(tokenExpiresIn)
			difference := exp.Sub(expected)
			Expect(difference < 10*time.Second).To(BeTrue())
		})

		It("contains the correct team", func() {
			Expect(claims.Team).To(Equal(params.Team))
		})

		It("contains the correct pipeline", func() {
			Expect(claims.Pipeline).To(Equal(params.Pipeline))
		})

		It("contains the correct instance vars", func() {
			Expect(claims.InstanceVars["foo"]).ToNot(BeEmpty())
			Expect(claims.InstanceVars["foo"]).To(Equal(params.InstanceVars["foo"]))
		})

		It("contains the correct job", func() {
			Expect(claims.Job).To(Equal(params.Job))
		})

		It("has no default audience", func() {
			Expect(claims.Audience).To(HaveLen(0))
		})
	})
})
