package token_test

import (
	"crypto/rand"
	"crypto/rsa"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/skymarshal/token"
	"golang.org/x/oauth2"
)

var _ = Describe("Token Generator", func() {

	Describe("Generate", func() {
		var tokenGenerator token.Generator

		Context("with invalid signing key", func() {
			BeforeEach(func() {
				tokenGenerator = token.NewGenerator(nil)
			})

			It("errors", func() {
				_, err := tokenGenerator.Generate(nil)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with a valid signing key", func() {
			var err error
			var signingKey *rsa.PrivateKey

			BeforeEach(func() {
				signingKey, err = rsa.GenerateKey(rand.Reader, 2048)
				Expect(err).NotTo(HaveOccurred())

				tokenGenerator = token.NewGenerator(signingKey)
			})

			Context("without claims", func() {
				It("errors", func() {
					_, err := tokenGenerator.Generate(map[string]interface{}{})
					Expect(err).To(HaveOccurred())
				})
			})

			Context("with claims", func() {
				var oauthToken *oauth2.Token

				JustBeforeEach(func() {
					claims := map[string]interface{}{
						"sub":   "1234567890",
						"exp":   2524608000,
						"teams": []string{"some-team"},
					}

					oauthToken, err = tokenGenerator.Generate(claims)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns a bearer token with an expiration", func() {
					Expect(oauthToken).NotTo(BeNil())
					Expect(oauthToken.AccessToken).NotTo(BeNil())
					Expect(oauthToken.TokenType).To(Equal("Bearer"))
					Expect(oauthToken.Expiry.Unix()).To(Equal(int64(2524608000)))
				})

				It("returns a signed jwt token", func() {
					var claims map[string]interface{}
					err := parse(oauthToken.AccessToken, signingKey, &claims)
					Expect(err).NotTo(HaveOccurred())
					Expect(claims).NotTo(BeNil())
				})

				It("returns a jwt token with claims", func() {
					var claims struct {
						Sub   string   `json:"sub"`
						Exp   int      `json:"exp"`
						Teams []string `json:"teams"`
					}
					err := parse(oauthToken.AccessToken, signingKey, &claims)
					Expect(err).NotTo(HaveOccurred())
					Expect(claims.Sub).To(Equal("1234567890"))
					Expect(claims.Exp).To(Equal(2524608000))
					Expect(claims.Teams).To(ContainElement("some-team"))
				})

				It("includes the claims in the token extras", func() {
					Expect(oauthToken.Extra("sub")).To(Equal("1234567890"))
					Expect(oauthToken.Extra("exp")).To(Equal(2524608000))
					Expect(oauthToken.Extra("teams")).To(ContainElement("some-team"))
				})
			})
		})
	})
})
