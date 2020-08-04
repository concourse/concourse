package db_test

import (
	"time"

	"github.com/concourse/concourse/atc/db"
	"gopkg.in/square/go-jose.v2/jwt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Access Token Lifecycle", func() {
	var (
		factory   db.AccessTokenFactory
		lifecycle db.AccessTokenLifecycle
	)

	BeforeEach(func() {
		factory = db.NewAccessTokenFactory(dbConn)
		lifecycle = db.NewAccessTokenLifecycle(dbConn)
	})

	It("removes expired access tokens", func() {
		By("having 2 expired tokens in the database")
		tomorrow := jwt.NewNumericDate(now().Add(24 * time.Hour))
		yesterday := jwt.NewNumericDate(now().Add(-24 * time.Hour))
		factory.CreateAccessToken("expiredToken1", db.Claims{
			Claims: jwt.Claims{Expiry: yesterday},
		})
		factory.CreateAccessToken("expiredToken2", db.Claims{
			Claims: jwt.Claims{Expiry: yesterday},
		})
		factory.CreateAccessToken("activeToken", db.Claims{
			Claims: jwt.Claims{Expiry: tomorrow},
		})

		By("removing expired tokens")
		n, err := lifecycle.RemoveExpiredAccessTokens(0)
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(2), "did not delete expired tokens")

		By("ensuring the active tokens are still present")
		_, found, _ := factory.GetAccessToken("activeToken")
		Expect(found).To(BeTrue(), "active token was removed")
	})

	It("respects the leeway for expiration time", func() {
		By("having a token that is 24 hours old")
		yesterday := jwt.NewNumericDate(now().Add(-24 * time.Hour))
		factory.CreateAccessToken("expiredToken", db.Claims{
			Claims: jwt.Claims{Expiry: yesterday},
		})

		By("removing expired tokens with leeway of 25 hours")
		n, err := lifecycle.RemoveExpiredAccessTokens(25 * time.Hour)
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(0), "did not respect leeway")
	})
})

func now() time.Time {
	var t time.Time
	dbConn.QueryRow("SELECT now()").Scan(&t)
	return t
}
