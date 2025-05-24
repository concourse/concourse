package idtoken_test

import (
	"time"

	"github.com/concourse/concourse/atc"
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
	var secrets creds.SecretsWithContext
	var context creds.SecretLookupContext

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

		context = creds.SecretLookupContext{
			Team:     "main",
			Pipeline: "idtoken",
			InstanceVars: atc.InstanceVars{
				"foo": "bar",
			},
			Job: "testjob",
		}
	})

	It("provides correct (empty) lookup path", func() {
		lookups := secrets.NewSecretLookupPathsWithContext(context, false)
		Expect(lookups).To(HaveLen(0))
	})

	It("returns a correct token for passed team+pipeline", func() {
		token, _, _, err := secrets.GetWithContext("token", context)
		Expect(err).ToNot(HaveOccurred())

		parsed, err := jwt.ParseSigned(token.(string))
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

		Expect(claims.Subject).To(Equal(context.Team + "/" + context.Pipeline))
		Expect(claims.Team).To(Equal(context.Team))
		Expect(claims.Pipeline).To(Equal(context.Pipeline))
		Expect(claims.InstanceVars.String()).To(Equal(context.InstanceVars.String()))
		Expect(claims.Job).To(Equal(context.Job))
	})

})
