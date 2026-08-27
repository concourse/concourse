package idtoken_test

import (
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/idtoken"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IDToken Secret", func() {

	var (
		tokenGenerator  idtoken.TokenGenerator
		verificationKey jose.JSONWebKey
		secrets         creds.Secrets
		params          creds.SecretLookupParams
	)

	BeforeEach(func() {
		signingKeyFake := &dbfakes.FakeSigningKey{}
		signingKeyFake.JWKReturns(*rsaJWK)
		signingKeyFake.CreatedAtReturns(time.Now())
		signingKeyFake.IDReturns(rsaJWK.KeyID)
		signingKeyFake.KeyTypeReturns(db.SigningKeyTypeRSA)

		verificationKey = rsaJWK.Public()

		signingKeyFactoryFake := &dbfakes.FakeSigningKeyFactory{}
		signingKeyFactoryFake.GetAllKeysReturns([]db.SigningKey{
			signingKeyFake,
		}, nil)

		signingKeyFactoryFake.GetNewestKeyReturns(signingKeyFake, nil)
		tokenGenerator = idtoken.TokenGenerator{
			Issuer:            testIssuer,
			SigningKeyFactory: signingKeyFactoryFake,
			ExpiresIn:         tokenExpiresIn,
		}
		secrets = &idtoken.IDToken{
			TokenGenerator: &tokenGenerator,
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

	It("provides exactly one lookup path", func() {
		lookups := secrets.NewSecretLookupPaths(params, false)
		Expect(lookups).To(HaveLen(1), "only returns one lookup path")
		Expect(lookups[0].VariableToSecretPath("token")).To(Equal("main/idtoken/foo:bar/testjob/token"))
	})

	It("returns a correct token for passed team/pipeline/job", func() {
		lookups := secrets.NewSecretLookupPaths(params, false)
		Expect(lookups).To(HaveLen(1))
		secretPath, err := lookups[0].VariableToSecretPath("token")
		Expect(err).ToNot(HaveOccurred())

		token, expiresIn, found, err := secrets.Get(secretPath, params)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(expiresIn).ToNot(BeNil())
		Expect(*expiresIn).To(BeTemporally("~", time.Now().Add(tokenExpiresIn), time.Second))

		parsed, err := jwt.ParseSigned(token.(string), []jose.SignatureAlgorithm{idtoken.DefaultAlgorithm})
		Expect(err).ToNot(HaveOccurred())

		type claimStruct struct {
			jwt.Claims
			Team         string           `json:"team"`
			Pipeline     string           `json:"pipeline"`
			InstanceVars atc.InstanceVars `json:"instance_vars"`
			Job          string           `json:"job"`
		}

		claims := claimStruct{}
		err = parsed.Claims(verificationKey, &claims)
		Expect(err).To(Succeed())

		Expect(claims.Subject).To(Equal(params.Team + "/" + params.Pipeline))
		Expect(claims.Team).To(Equal(params.Team))
		Expect(claims.Pipeline).To(Equal(params.Pipeline))
		Expect(claims.InstanceVars.String()).To(Equal(params.InstanceVars.String()))
		Expect(claims.Job).To(Equal(params.Job))
	})

	Context("when using a custom OIDC issuer", func() {
		var customIssuer string

		BeforeEach(func() {
			customIssuer = "https://oidc.example.com"
			tokenGenerator.Issuer = customIssuer
		})

		It("generates token with custom issuer in iss claim", func() {
			lookups := secrets.NewSecretLookupPaths(params, false)
			Expect(lookups).To(HaveLen(1))
			secretPath, err := lookups[0].VariableToSecretPath("token")

			Expect(err).ToNot(HaveOccurred())
			token, _, _, err := secrets.Get(secretPath, params)
			Expect(err).ToNot(HaveOccurred())

			parsed, err := jwt.ParseSigned(token.(string), []jose.SignatureAlgorithm{idtoken.DefaultAlgorithm})
			Expect(err).ToNot(HaveOccurred())

			claims := jwt.Claims{}
			err = parsed.Claims(verificationKey, &claims)
			Expect(err).To(Succeed())

			Expect(claims.Issuer).To(Equal(customIssuer))
		})
	})

	It("errors when a field other than 'token' is used", func() {
		lookups := secrets.NewSecretLookupPaths(params, false)
		Expect(lookups).To(HaveLen(1))
		secretPath, err := lookups[0].VariableToSecretPath("some-other-field")
		Expect(err).ToNot(HaveOccurred())

		token, expiresIn, found, err := secrets.Get(secretPath, params)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("idtoken credential provider only supports the field 'token'"))
		Expect(token).To(BeNil())
		Expect(expiresIn).To(BeNil())
		Expect(found).To(BeFalse())
	})

	It("errors when params is empty", func() {
		params = creds.SecretLookupParams{}
		lookups := secrets.NewSecretLookupPaths(params, false)
		Expect(lookups).To(HaveLen(1))
		secretPath, err := lookups[0].VariableToSecretPath("token")
		Expect(err).ToNot(HaveOccurred())

		token, expiresIn, found, err := secrets.Get(secretPath, params)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("idtoken credential provider was called with empty params"))
		Expect(token).To(BeNil())
		Expect(expiresIn).To(BeNil())
		Expect(found).To(BeFalse())
	})

	It("errors when different params are passed to Secrets.NewSecretLookupPaths() and Secrets.Get()", func() {
		lookups := secrets.NewSecretLookupPaths(params, false)
		Expect(lookups).To(HaveLen(1))
		secretPath, err := lookups[0].VariableToSecretPath("token")
		Expect(err).ToNot(HaveOccurred())

		params = creds.SecretLookupParams{
			Team:         "main",
			Pipeline:     "idtoken",
			InstanceVars: atc.InstanceVars{},
			Job:          "other-job",
		}

		token, expiresIn, found, err := secrets.Get(secretPath, params)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("idtoken credential provider was called with different secret params"))
		Expect(token).To(BeNil())
		Expect(expiresIn).To(BeNil())
		Expect(found).To(BeFalse())
	})
})
