package idtoken_test

import (
	"time"

	"github.com/concourse/concourse/atc/creds/idtoken"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	testIssuer     = "https://concourse.test"
	testTeam       = "main"
	testPipeline   = "idtoken"
	tokenExpiresIn = 15 * time.Minute
)

var _ = Describe("IDToken TokenGenerator", func() {
	var rsaSigningKey db.SigningKey
	var rsaVerificationKey jose.JSONWebKey
	var signingKeyFactory db.SigningKeyFactory
	var tokenGenerator *idtoken.TokenGenerator

	BeforeEach(func() {
		signingKeyFake := &dbfakes.FakeSigningKey{}
		signingKeyFake.JWKReturns(*rsaJWK)
		signingKeyFake.CreatedAtReturns(time.Now())
		signingKeyFake.IDReturns(rsaJWK.KeyID)
		signingKeyFake.KeyTypeReturns(db.SigningKeyTypeRSA)
		rsaSigningKey = signingKeyFake

		rsaVerificationKey = rsaJWK.Public()

		signingKeyFactoryFake := &dbfakes.FakeSigningKeyFactory{}
		signingKeyFactoryFake.GetAllKeysReturns([]db.SigningKey{
			rsaSigningKey,
		}, nil)
		signingKeyFactoryFake.GetNewestKeyReturns(rsaSigningKey, nil)
		signingKeyFactory = signingKeyFactoryFake

		tokenGenerator = &idtoken.TokenGenerator{
			Issuer:            testIssuer,
			SigningKeyFactory: signingKeyFactory,
			ExpiresIn:         tokenExpiresIn,
		}
	})

	It("generates a valid token", func() {
		token, _, err := tokenGenerator.GenerateToken(testTeam, testPipeline)
		Expect(err).ToNot(HaveOccurred())

		parsed, err := jwt.ParseSigned(token)
		Expect(err).ToNot(HaveOccurred())

		claims := jwt.Claims{}
		err = parsed.Claims(rsaVerificationKey, &claims)
		Expect(err).To(Succeed())
	})

	It("respects subject scope team", func() {
		tokenGenerator.SubjectScope = idtoken.SubjectScopeTeam
		token, _, err := tokenGenerator.GenerateToken(testTeam, testPipeline)
		Expect(err).ToNot(HaveOccurred())

		parsed, err := jwt.ParseSigned(token)
		Expect(err).ToNot(HaveOccurred())

		claims := jwt.Claims{}
		err = parsed.Claims(rsaVerificationKey, &claims)
		Expect(err).To(Succeed())

		Expect(claims.Subject).To(Equal(testTeam))
	})

	It("escapes team names safely", func() {
		tokenGenerator.SubjectScope = idtoken.SubjectScopeTeam
		token, _, err := tokenGenerator.GenerateToken("fake/team", "pipeline")
		Expect(err).ToNot(HaveOccurred())

		parsed, err := jwt.ParseSigned(token)
		Expect(err).ToNot(HaveOccurred())

		claims := jwt.Claims{}
		err = parsed.Claims(rsaVerificationKey, &claims)
		Expect(err).To(Succeed())

		Expect(claims.Subject).To(Equal("fake%2Fteam"))
	})

	It("adds aud claim when requested", func() {
		tokenGenerator.Audience = []string{"testaud"}
		token, _, err := tokenGenerator.GenerateToken(testTeam, testPipeline)
		Expect(err).ToNot(HaveOccurred())

		parsed, err := jwt.ParseSigned(token)
		Expect(err).ToNot(HaveOccurred())

		claims := jwt.Claims{}
		err = parsed.Claims(rsaVerificationKey, &claims)
		Expect(err).To(Succeed())

		Expect(claims.Audience).To(ContainElement("testaud"))
	})

	Context("Generated Token", func() {
		type claimStruct struct {
			jwt.Claims
			Team     string `json:"team"`
			Pipeline string `json:"pipeline"`
		}

		var generatedToken string
		var claims claimStruct
		var generatedAt time.Time

		BeforeEach(func() {
			var err error
			generatedToken, _, err = tokenGenerator.GenerateToken(testTeam, testPipeline)
			Expect(err).ToNot(HaveOccurred())

			generatedAt = time.Now()

			parsed, err := jwt.ParseSigned(generatedToken)
			Expect(err).ToNot(HaveOccurred())

			err = parsed.Claims(rsaVerificationKey, &claims)
			Expect(err).To(Succeed())
		})

		It("contains the correct subject", func() {
			Expect(claims.Subject).To(Equal(testTeam + "/" + testPipeline))
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
			Expect(claims.Team).To(Equal(testTeam))
		})

		It("contains the correct pipeline", func() {
			Expect(claims.Pipeline).To(Equal(testPipeline))
		})

		It("has no default audience", func() {
			Expect(claims.Audience).To(HaveLen(0))
		})
	})
})
