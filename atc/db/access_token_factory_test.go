package db_test

import (
	"github.com/concourse/concourse/atc/db"

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
		err := factory.CreateAccessToken("my-awesome-token", db.Claims{
			Sub:       "hello",
			ExpiresAt: 1234567890,
			Extra: map[string]interface{}{
				"groups": []interface{}{"group1", "group2"},
			},
		})
		Expect(err).ToNot(HaveOccurred())

		token, ok, _ := factory.GetAccessToken("my-awesome-token")
		Expect(ok).To(BeTrue())
		Expect(token.Token()).To(Equal("my-awesome-token"))
		Expect(token.Claims()).To(Equal(db.Claims{
			Sub:       "hello",
			ExpiresAt: 1234567890,
			Extra: map[string]interface{}{
				"groups": []interface{}{"group1", "group2"},
			},
		}))
	})
})
