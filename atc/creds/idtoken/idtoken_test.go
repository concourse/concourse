package idtoken_test

import (
	"time"

	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/idtoken"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IDToken Secret", func() {

	var tokenGenerator idtoken.TokenGenerator
	var verificationKey jose.JSONWebKey
	var secrets creds.Secrets

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
	})

	It("provides correct (faked) lookup path", func() {
		lookups := secrets.NewSecretLookupPaths(testTeam, testPipeline, false)
		Expect(lookups).To(HaveLen(1))

		lookup := lookups[0]
		_, err := lookup.VariableToSecretPath("other")
		Expect(err).To(HaveOccurred())

		path, err := lookup.VariableToSecretPath("token")
		Expect(err).ToNot(HaveOccurred())
		Expect(path).To(Equal(testTeam + "/" + testPipeline))
	})

	It("returns a correct token for passed team+pipeline", func() {
		lookups := secrets.NewSecretLookupPaths(testTeam, testPipeline, false)
		path, err := lookups[0].VariableToSecretPath("token")
		Expect(err).ToNot(HaveOccurred())

		token, _, _, err := secrets.Get(path)
		Expect(err).ToNot(HaveOccurred())

		parsed, err := jwt.ParseSigned(token.(string))
		Expect(err).ToNot(HaveOccurred())

		type claimStruct struct {
			jwt.Claims
			Team     string `json:"team"`
			Pipeline string `json:"pipeline"`
		}

		claims := claimStruct{}
		err = parsed.Claims(verificationKey, &claims)
		Expect(err).To(Succeed())

		Expect(claims.Subject).To(Equal(testTeam + "/" + testPipeline))
		Expect(claims.Team).To(Equal(testTeam))
		Expect(claims.Pipeline).To(Equal(testPipeline))
	})

})
