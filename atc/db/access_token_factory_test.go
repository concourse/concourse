package db_test

import (
	"github.com/concourse/concourse/atc/db"
	"gopkg.in/square/go-jose.v2/jwt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Access Token Factory", func() {
	var (
		factory db.AccessTokenFactory
	)

	BeforeEach(func() {
		factory = db.NewAccessTokenFactory(dbConn)
	})

	It("can create and fetch access tokens", func() {
		date := jwt.NumericDate(1234567890)
		err := factory.CreateAccessToken("my-awesome-token", db.Claims{
			RawClaims: map[string]interface{}{
				"iss":    "issuer",
				"sub":    "subject",
				"aud":    []interface{}{"audience"},
				"exp":    date,
				"nbf":    date,
				"iat":    date,
				"jti":    "id",
				"groups": []interface{}{"group1", "group2"},
			},
		})
		Expect(err).ToNot(HaveOccurred())

		token, ok, _ := factory.GetAccessToken("my-awesome-token")
		Expect(ok).To(BeTrue())
		Expect(token.Token).To(Equal("my-awesome-token"))
		Expect(token.Claims).To(Equal(db.Claims{
			Claims: jwt.Claims{
				Issuer:    "issuer",
				Subject:   "subject",
				Audience:  []string{"audience"},
				Expiry:    &date,
				NotBefore: &date,
				IssuedAt:  &date,
				ID:        "id",
			},
			RawClaims: map[string]interface{}{
				"iss":    "issuer",
				"sub":    "subject",
				"aud":    []interface{}{"audience"},
				"exp":    float64(date),
				"nbf":    float64(date),
				"iat":    float64(date),
				"jti":    "id",
				"groups": []interface{}{"group1", "group2"},
			},
		}))
	})
})
