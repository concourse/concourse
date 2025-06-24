package api_test

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-jose/go-jose/v3"

	"github.com/concourse/concourse/atc/creds/idtoken"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IDToken API", func() {
	Describe("GET /.well-known/openid-configuration", func() {
		type openidConfig struct {
			Issuer  string `json:"issuer"`
			JWKSURI string `json:"jwks_uri"`
		}
		var response *http.Response
		var info openidConfig

		JustBeforeEach(func() {
			var err error
			response, err = client.Get(server.URL + "/.well-known/openid-configuration")
			Expect(err).NotTo(HaveOccurred())
			json.NewDecoder(response.Body).Decode(&info)
		})

		It("returns Content-Type 'application/json'", func() {
			expectedHeaderEntries := map[string]string{
				"Content-Type": "application/json",
			}
			Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
		})

		It("contains correct issuer", func() {
			Expect(info.Issuer).To(Equal(externalURL))
		})

		It("contains correct issuer", func() {
			Expect(info.JWKSURI).To(Equal(externalURL + "/.well-known/jwks.json"))
		})
	})

	Describe("GET /.well-known/jwks.json", func() {

		var rsaKey dbfakes.FakeSigningKey
		var ecKey dbfakes.FakeSigningKey
		var response *http.Response
		var jwks jose.JSONWebKeySet

		JustBeforeEach(func() {
			rsaJWK, _ := idtoken.GenerateNewKey(db.SigningKeyTypeRSA)
			rsaKey = dbfakes.FakeSigningKey{}
			rsaKey.CreatedAtReturns(time.Now())
			rsaKey.IDReturns(rsaJWK.KeyID)
			rsaKey.JWKReturns(*rsaJWK)

			ecJWK, _ := idtoken.GenerateNewKey(db.SigningKeyTypeEC)
			ecKey = dbfakes.FakeSigningKey{}
			ecKey.CreatedAtReturns(time.Now())
			ecKey.IDReturns(ecJWK.KeyID)
			ecKey.JWKReturns(*ecJWK)

			dbSigningKeyFactory.GetAllKeysStub = func() ([]db.SigningKey, error) {
				return []db.SigningKey{&rsaKey, &ecKey}, nil
			}

			var err error
			response, err = client.Get(server.URL + "/.well-known/jwks.json")
			Expect(err).NotTo(HaveOccurred())
			json.NewDecoder(response.Body).Decode(&jwks)
		})

		It("returns Content-Type 'application/json'", func() {
			expectedHeaderEntries := map[string]string{
				"Content-Type": "application/json",
			}
			Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
		})

		It("contains correct keys", func() {
			Expect(jwks.Keys).To(HaveLen(2))
			Expect(jwks.Keys[0].KeyID).To(Equal(rsaKey.ID()))
			Expect(jwks.Keys[1].KeyID).To(Equal(ecKey.ID()))
		})

		It("does not contain private keys", func() {
			for _, key := range jwks.Keys {
				Expect(key.IsPublic()).To(BeTrue())
			}
		})

	})
})
