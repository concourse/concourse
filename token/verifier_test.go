package token_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/skymarshal/token"
	"github.com/dgrijalva/jwt-go"
	"golang.org/x/oauth2"
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
				dexIssuerUrl = dexServer.URL() + "/sky/dex"

				var err error
				signingKey, err = rsa.GenerateKey(rand.Reader, 2048)
				Expect(err).NotTo(HaveOccurred())

				dexServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/sky/dex/.well-known/openid-configuration"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]interface{}{
							"issuer":         dexIssuerUrl,
							"jwks_uri":       dexIssuerUrl + "/keys",
							"token_endpoint": dexIssuerUrl + "/token",
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/sky/dex/keys"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]interface{}{
							"keys": []map[string]string{{
								"kty": "RSA",
								"n":   n(&signingKey.PublicKey),
								"e":   e(&signingKey.PublicKey),
							}},
						}),
					),
				)

				jwtClaims := jwt.MapClaims(map[string]interface{}{
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
				})

				jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwtClaims)
				signedToken, _ := jwtToken.SignedString(signingKey)

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
