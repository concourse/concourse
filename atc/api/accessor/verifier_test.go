package accessor_test

import (
	"errors"
	"net/http"
	"time"

	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/concourse/concourse/atc/api/accessor"
)

var _ = Describe("Verifier", func() {
	var (
		accessTokenFetcher *accessorfakes.FakeAccessTokenFetcher
		accessToken        db.AccessToken

		req *http.Request

		verifier accessor.TokenVerifier

		err error
	)

	BeforeEach(func() {
		accessTokenFetcher = new(accessorfakes.FakeAccessTokenFetcher)
		accessTokenFetcher.GetAccessTokenCalls(func(string) (db.AccessToken, bool, error) {
			return accessToken, true, nil
		})

		req, _ = http.NewRequest("GET", "localhost:8080", nil)
		req.Header.Set("Authorization", "bearer 1234567890")

		verifier = accessor.NewVerifier(accessTokenFetcher, []string{"some-aud"})
	})

	Describe("Verify", func() {

		JustBeforeEach(func() {
			_, err = verifier.Verify(req)
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
				req.Header.Set("Authorization", "invalid")
			})

			It("fails verification", func() {
				Expect(err).To(Equal(accessor.ErrVerificationInvalidToken))
			})
		})

		Context("when request has an invalid token type", func() {
			BeforeEach(func() {
				req.Header.Set("Authorization", "not-bearer 1234567890")
			})

			It("fails verification", func() {
				Expect(err).To(Equal(accessor.ErrVerificationInvalidToken))
			})
		})

		Context("when getting the access token errors", func() {
			BeforeEach(func() {
				accessTokenFetcher.GetAccessTokenReturns(db.AccessToken{}, false, errors.New("db error"))
			})

			It("errors", func() {
				Expect(err).To(MatchError("db error"))
			})
		})

		Context("when the token is not found in the DB", func() {
			BeforeEach(func() {
				accessTokenFetcher.GetAccessTokenReturns(db.AccessToken{}, false, nil)
			})

			It("fails verification", func() {
				Expect(err).To(Equal(accessor.ErrVerificationInvalidToken))
			})
		})

		Context("when the claims have expired", func() {
			BeforeEach(func() {
				oneHourAgo := jwt.NewNumericDate(time.Now().Add(-1 * time.Hour))
				accessToken.Claims = db.Claims{
					Claims: jwt.Claims{
						Expiry: oneHourAgo,
					},
				}
			})

			It("fails verification", func() {
				Expect(err).To(Equal(accessor.ErrVerificationTokenExpired))
			})
		})

		Context("whne the claims have invalid audience", func() {
			BeforeEach(func() {
				oneHourFromNow := jwt.NewNumericDate(time.Now().Add(1 * time.Hour))
				accessToken.Claims = db.Claims{
					Claims: jwt.Claims{
						Expiry:   oneHourFromNow,
						Audience: []string{"invalid"},
					},
				}
			})

			It("fails verification", func() {
				Expect(err).To(Equal(accessor.ErrVerificationInvalidAudience))
			})
		})

		Context("when the claims are valid", func() {
			BeforeEach(func() {
				oneHourFromNow := jwt.NewNumericDate(time.Now().Add(1 * time.Hour))
				accessToken.Claims = db.Claims{
					Claims: jwt.Claims{
						Expiry:   oneHourFromNow,
						Audience: []string{"some-aud"},
					},
				}
			})

			It("succeeds", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
