package token_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/concourse/skymarshal/token"
	"golang.org/x/oauth2"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

var _ = Describe("Token Verifier", func() {
	Describe("Verify", func() {
		var tokenVerifier token.Verifier

		Context("without a client id (audience)", func() {
			BeforeEach(func() {
				tokenVerifier = token.NewVerifier("", "http://example.com")
			})

			It("errors", func() {
				_, err := tokenVerifier.Verify(context.Background(), &oauth2.Token{})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("without an issuer url", func() {
			BeforeEach(func() {
				tokenVerifier = token.NewVerifier("client-id", "")
			})

			It("errors", func() {
				_, err := tokenVerifier.Verify(context.Background(), &oauth2.Token{})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("without a context", func() {
			BeforeEach(func() {
				tokenVerifier = token.NewVerifier("client-id", "http://example.com")
			})

			It("errors", func() {
				_, err := tokenVerifier.Verify(nil, &oauth2.Token{})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("without an id_token inside the oauth token", func() {
			BeforeEach(func() {
				tokenVerifier = token.NewVerifier("client-id", "http://example.com")
			})

			It("errors", func() {
				_, err := tokenVerifier.Verify(context.Background(), &oauth2.Token{})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with a valid oauth token", func() {
			var (
				dexIssuerUrl   string
				dexServer      *ghttp.Server
				signingKey     *rsa.PrivateKey
				verifiedClaims *token.VerifiedClaims
			)

			BeforeEach(func() {
				dexServer = ghttp.NewServer()
				dexIssuerUrl = dexServer.URL() + "/sky/issuer"

				var err error
				signingKey, err = rsa.GenerateKey(rand.Reader, 2048)
				Expect(err).NotTo(HaveOccurred())

				dexServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/sky/issuer/.well-known/openid-configuration"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]interface{}{
							"issuer":         dexIssuerUrl,
							"jwks_uri":       dexIssuerUrl + "/keys",
							"token_endpoint": dexIssuerUrl + "/token",
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/sky/issuer/keys"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]interface{}{
							"keys": []map[string]string{{
								"kty": "RSA",
								"n":   n(&signingKey.PublicKey),
								"e":   e(&signingKey.PublicKey),
							}},
						}),
					),
				)

				claims := map[string]interface{}{
					"iss":   dexIssuerUrl,
					"aud":   "client-id",
					"sub":   "dex-sub",
					"exp":   "2524608000",
					"name":  "Firstname Lastname",
					"email": "my@email.com",
					"federated_claims": map[string]string{
						"user_id":      "my-user-id",
						"user_name":    "my-user-name",
						"connector_id": "my-connector-id",
					},
					"groups": []string{
						"group-0", "group-1", "group-2",
					},
				}

				options := &jose.SignerOptions{}
				options = options.WithType("JWT")

				signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: signingKey}, options)
				Expect(err).NotTo(HaveOccurred())

				signedToken, err := jwt.Signed(signer).Claims(claims).CompactSerialize()
				Expect(err).NotTo(HaveOccurred())

				oauthToken := (&oauth2.Token{}).WithExtra(map[string]interface{}{
					"id_token": signedToken,
				})

				tokenVerifier := token.NewVerifier("client-id", dexIssuerUrl)
				verifiedClaims, err = tokenVerifier.Verify(context.Background(), oauthToken)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				dexServer.Close()
			})

			It("returns the expected verified claims", func() {
				Expect(verifiedClaims.Sub).To(Equal("dex-sub"))
				Expect(verifiedClaims.Email).To(Equal("my@email.com"))
				Expect(verifiedClaims.Name).To(Equal("Firstname Lastname"))
				Expect(verifiedClaims.UserID).To(Equal("my-user-id"))
				Expect(verifiedClaims.UserName).To(Equal("my-user-name"))
				Expect(verifiedClaims.ConnectorID).To(Equal("my-connector-id"))
				Expect(verifiedClaims.Groups[0]).To(Equal("group-0"))
				Expect(verifiedClaims.Groups[1]).To(Equal("group-1"))
				Expect(verifiedClaims.Groups[2]).To(Equal("group-2"))
			})
		})
	})
})
