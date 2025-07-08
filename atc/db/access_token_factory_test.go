package db_test

import (
	"github.com/concourse/concourse/atc/db"
	"github.com/go-jose/go-jose/v3/jwt"

	. "github.com/onsi/ginkgo/v2"
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
			RawClaims: map[string]any{
				"iss": "issuer",
				"sub": "subject",
				"aud": []any{"audience"},
				"exp": date,
				"nbf": date,
				"iat": date,
				"jti": "id",

				"federated_claims": map[string]any{
					"user_id":      "userid",
					"connector_id": "github",
					"other":        "blah",
				},

				"groups": []any{"group1", "group2"},
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
			FederatedClaims: db.FederatedClaims{
				UserID:    "userid",
				Connector: "github",
			},
			RawClaims: map[string]any{
				"iss": "issuer",
				"sub": "subject",
				"aud": []any{"audience"},
				"exp": float64(date),
				"nbf": float64(date),
				"iat": float64(date),
				"jti": "id",

				"federated_claims": map[string]any{
					"user_id":      "userid",
					"connector_id": "github",
					"other":        "blah",
				},

				"groups": []any{"group1", "group2"},
			},
		}))
	})
	It("can delete access tokens", func() {
		err := factory.CreateAccessToken("my-delete-token", db.Claims{
			RawClaims: map[string]any{"sub": "subject"},
		})
		Expect(err).ToNot(HaveOccurred())

		token, ok, err := factory.GetAccessToken("my-delete-token")
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(token.Token).To(Equal("my-delete-token"))

		err = factory.DeleteAccessToken("my-delete-token")
		Expect(err).ToNot(HaveOccurred())

		_, ok, err = factory.GetAccessToken("my-delete-token")
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeFalse())
	})
})
