package accessor_test

import (
	"errors"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ClaimsCacher", func() {
	var (
		fakeAccessTokenFetcher *accessorfakes.FakeAccessTokenFetcher
		maxCacheSizeBytes      int

		claimsCacher accessor.AccessTokenFetcher
	)

	BeforeEach(func() {
		fakeAccessTokenFetcher = new(accessorfakes.FakeAccessTokenFetcher)
		maxCacheSizeBytes = 1000
	})

	JustBeforeEach(func() {
		claimsCacher = accessor.NewClaimsCacher(fakeAccessTokenFetcher, maxCacheSizeBytes)
	})

	It("fetches claims from the DB", func() {
		claimsCacher.GetAccessToken("token")
		Expect(fakeAccessTokenFetcher.GetAccessTokenCallCount()).To(Equal(1), "did not fetch from DB")
	})

	It("doesn't fetch from the DB when the result is cached", func() {
		claimsCacher.GetAccessToken("token")
		claimsCacher.GetAccessToken("token")
		Expect(fakeAccessTokenFetcher.GetAccessTokenCallCount()).To(Equal(1), "did not cache claims")
	})

	It("doesn't cache claims when cache size is exceeded", func() {
		fakeAccessTokenFetcher.GetAccessTokenReturns(db.AccessToken{
			Claims: db.Claims{RawClaims: map[string]interface{}{"a": stringWithLen(2000)}},
		}, true, nil)
		claimsCacher.GetAccessToken("token")
		claimsCacher.GetAccessToken("token")
		Expect(fakeAccessTokenFetcher.GetAccessTokenCallCount()).To(Equal(2), "cached claims that exceed length")
	})

	It("evicts the least recently used access token when size limit exceeded", func() {
		fakeAccessTokenFetcher.GetAccessTokenReturns(db.AccessToken{
			Claims: db.Claims{RawClaims: map[string]interface{}{"a": stringWithLen(400)}},
		}, true, nil)

		By("filling the cache")
		claimsCacher.GetAccessToken("token1")
		claimsCacher.GetAccessToken("token2")
		Expect(fakeAccessTokenFetcher.GetAccessTokenCallCount()).To(Equal(2))

		By("overflowing the cache")
		claimsCacher.GetAccessToken("token3")
		Expect(fakeAccessTokenFetcher.GetAccessTokenCallCount()).To(Equal(3))

		By("fetching the least recently used token")
		claimsCacher.GetAccessToken("token1")
		Expect(fakeAccessTokenFetcher.GetAccessTokenCallCount()).To(Equal(4), "did not evict least recently used")

		By("ensuring the latest token was not evicted")
		claimsCacher.GetAccessToken("token3")
		Expect(fakeAccessTokenFetcher.GetAccessTokenCallCount()).To(Equal(4), "evicted the latest token")
	})

	It("errors when the DB fails", func() {
		fakeAccessTokenFetcher.GetAccessTokenReturns(db.AccessToken{}, false, errors.New("error"))
		_, _, err := claimsCacher.GetAccessToken("token")
		Expect(err).To(HaveOccurred())
	})
})

func stringWithLen(l int) string {
	b := make([]byte, l)
	for i := 0; i < l; i++ {
		b[i] = 'a'
	}
	return string(b)
}