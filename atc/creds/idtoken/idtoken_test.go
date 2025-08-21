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

	var tokenGenerator idtoken.TokenGenerator
	var verificationKey jose.JSONWebKey
	var secrets creds.SecretsWithParams
	var params creds.SecretLookupParams

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

	It("provides correct (empty) lookup path", func() {
		lookups := secrets.NewSecretLookupPathsWithParams(params, false)
		Expect(lookups).To(HaveLen(0))
	})

	It("returns a correct token for passed team+pipeline", func() {
		token, _, _, err := secrets.GetWithParams("token", params)
		Expect(err).ToNot(HaveOccurred())

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

})
