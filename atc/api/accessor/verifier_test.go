package accessor_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/onsi/gomega/ghttp"
	"gopkg.in/square/go-jose.v2"
)

var _ = Describe("Verifier", func() {
	var (
		key        *rsa.PrivateKey
		authServer *ghttp.Server

		req      *http.Request
		verifier accessor.Verifier

		err    error
		claims map[string]interface{}
	)

	BeforeEach(func() {

		key, err = rsa.GenerateKey(rand.Reader, 2048)
		Expect(err).NotTo(HaveOccurred())

		authServer = ghttp.NewServer()
		authServer.SetAllowUnhandledRequests(true)

		req, err = http.NewRequest("GET", "localhost:8080", nil)
		Expect(err).NotTo(HaveOccurred())

		keysURL, _ := url.Parse(authServer.URL() + "/keys")
		verifier = accessor.NewVerifier(http.DefaultClient, keysURL, []string{"some-aud"})
	})

	AfterEach(func() {
		authServer.Close()
	})

	Describe("Verify", func() {

		JustBeforeEach(func() {
			claims, err = verifier.Verify(req)
		})

		Context("when request has no token", func() {
			BeforeEach(func() {
				req.Header.Del("Authorization")
			})

			It("fails with no token", func() {
				Expect(err).To(Equal(accessor.ErrVerificationNoToken))
			})
		})

		Context("when request has an invalid auth header", func() {
			BeforeEach(func() {
				req.Header.Add("Authorization", "1234567890")
			})

			It("fails verification", func() {
				Expect(err).To(Equal(accessor.ErrVerificationInvalidToken))
			})
		})

		Context("when request has an invalid token type", func() {
			BeforeEach(func() {
				req.Header.Add("Authorization", "not-bearer 1234567890")
			})

			It("fails verification", func() {
				Expect(err).To(Equal(accessor.ErrVerificationInvalidToken))
			})
		})

		Context("when request has an invalid token", func() {
			BeforeEach(func() {
				req.Header.Add("Authorization", fmt.Sprintf("bearer %s", "29384q29jdhkwjdhs"))
			})

			It("fails verification", func() {
				Expect(err).To(Equal(accessor.ErrVerificationInvalidToken))
			})
		})

		Context("when request has a token with an invalid signature", func() {
			BeforeEach(func() {
				req.Header.Add("Authorization", fmt.Sprintf("bearer %s", "eyJhbGciOiJSUzI1NiIsImtpZCI6ImtpZCJ9.eyJzdWIiOiAic29tZS1zdWIiLCAiZXhwIjogMH0.eiDPnv44MuLYfL9K0H6METeKDQSzmrSmUHAKxpXSZTIXa20VJurNMeBUF9uG4sAMoeNKlE4UEHrcn4xNtg8iwGqSMpLUNtVpuZFogKL3TFjhBha9LTNoH3uP5jjZB0MXMXu_xc9DM9qZnP7Efrm8zmDY7AGaK13sSVrneHbQ2VufsnzYxro1kXCyw5_QEyyemTrMLLyFdfe6XmZa20O4YthZor53vR9Iuaq1rrtTbYCiMIzVMdRrnX2B5FAMLqJso7XajKa5U9mTipW_YPHu8YOlUuu8HeuvmhrotEy5uD8HUAmVdkOIlKkP661cDVAl-HfcpVtCBmAGLFTSH-ANJw"))
			})

			Context("when the auth server responds with an error", func() {
				BeforeEach(func() {
					authServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/keys"),
							ghttp.RespondWith(http.StatusInternalServerError, nil),
						),
					)
				})

				It("tries to fetch a public key from the auth server", func() {
					Expect(authServer.ReceivedRequests()).To(HaveLen(1))
				})

				It("fails verification", func() {
					Expect(err).To(Equal(accessor.ErrVerificationFetchFailed))
				})
			})

			Context("when the auth server responds with a valid public key", func() {
				BeforeEach(func() {
					authServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/keys"),
							ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]interface{}{
								"keys": []map[string]string{{
									"kty": "RSA",
									"kid": "kid",
									"n":   n(&key.PublicKey),
									"e":   e(&key.PublicKey),
								}},
							}),
						),
					)
				})

				It("tries to fetch a public key from the auth server", func() {
					Expect(authServer.ReceivedRequests()).To(HaveLen(1))
				})

				It("fails verification", func() {
					Expect(err).To(Equal(accessor.ErrVerificationInvalidSignature))
				})
			})
		})


		Context("when the token has a valid signature", func() {
			BeforeEach(func() {
				authServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/keys"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]interface{}{
							"keys": []map[string]string{{
								"kty": "RSA",
								"kid": "kid",
								"n":   n(&key.PublicKey),
								"e":   e(&key.PublicKey),
							}},
						}),
					),
				)
			})

			Context("when the token is expired", func() {
				BeforeEach(func() {
					token := newToken(key, `{"sub": "some-sub", "exp": 0}`)

					req.Header.Add("Authorization", fmt.Sprintf("bearer %s", token))
				})

				It("fails verification", func() {
					Expect(err).To(Equal(accessor.ErrVerificationTokenExpired))
				})
			})

			Context("when the token has an invalid audience", func() {
				BeforeEach(func() {
					token := newToken(key, `{"sub": "some-sub", "exp": 9999999999, "aud": "not-aud"}`)

					req.Header.Add("Authorization", fmt.Sprintf("bearer %s", token))
				})

				It("fails verification", func() {
					Expect(err).To(Equal(accessor.ErrVerificationInvalidAudience))
				})
			})

			Context("when the token has valid claims", func() {

				BeforeEach(func() {
					token := newToken(key, `{"sub": "some-sub", "exp": 9999999999, "aud": "some-aud"}`)

					req.Header.Add("Authorization", fmt.Sprintf("bearer %s", token))
				})

				It("verifies the token", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(claims).Should(HaveKeyWithValue("sub", "some-sub"))
					Expect(claims).Should(HaveKeyWithValue("aud", "some-aud"))
				})

				Context("when handling multiple verifications", func() {
					BeforeEach(func() {
						claims, err := verifier.Verify(req)

						Expect(err).NotTo(HaveOccurred())
						Expect(claims).Should(HaveKeyWithValue("sub", "some-sub"))
						Expect(claims).Should(HaveKeyWithValue("aud", "some-aud"))

						claims, err = verifier.Verify(req)

						Expect(err).NotTo(HaveOccurred())
						Expect(claims).Should(HaveKeyWithValue("sub", "some-sub"))
						Expect(claims).Should(HaveKeyWithValue("aud", "some-aud"))
					})

					It("only fetches the public key once", func() {
						Expect(authServer.ReceivedRequests()).To(HaveLen(1))
					})
				})
			})
		})
	})
})

func n(pub *rsa.PublicKey) string {
	return encode(pub.N.Bytes())
}

func e(pub *rsa.PublicKey) string {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, uint64(pub.E))
	return encode(bytes.TrimLeft(data, "\x00"))
}

func encode(payload []byte) string {
	result := base64.URLEncoding.EncodeToString(payload)
	return strings.TrimRight(result, "=")
}

func newToken(key *rsa.PrivateKey, claims string) string {

	signingKey := jose.SigningKey{
		Algorithm: jose.RS256,
		Key:       key,
	}

	var signerOpts = &jose.SignerOptions{}
	signerOpts.WithHeader("kid", "kid")

	signer, err := jose.NewSigner(signingKey, signerOpts)
	Expect(err).NotTo(HaveOccurred())

	object, err := signer.Sign([]byte(claims))
	Expect(err).NotTo(HaveOccurred())

	token, err := object.CompactSerialize()
	Expect(err).NotTo(HaveOccurred())

	return token
}
