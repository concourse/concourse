package creds_test

import (
	"fmt"
	"time"

	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/credsfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func makeGetStub(name string, value any, expiration *time.Time, found bool, err error) (
	func(string, creds.SecretLookupParams) (any, *time.Time, bool, error),
	*int, *int,
) {
	reads := 0
	misses := 0
	return func(secretPath string, _ creds.SecretLookupParams) (any, *time.Time, bool, error) {
		if secretPath == name {
			reads++
			return value, expiration, found, err
		}
		misses++
		return nil, nil, false, nil
	}, &reads, &misses
}

var _ = Describe("Caching of secrets", func() {

	var (
		secretManager       *credsfakes.FakeSecrets
		cacheConfig         creds.SecretCacheConfig
		cachedSecretManager *creds.CachedSecrets
		secretParams        creds.SecretLookupParams
		underlyingReads     *int
		underlyingMisses    *int
	)

	BeforeEach(func() {
		secretManager = new(credsfakes.FakeSecrets)
		cacheConfig = creds.SecretCacheConfig{
			Duration:         400 * time.Millisecond,
			DurationNotFound: 200 * time.Millisecond,
			PurgeInterval:    100 * time.Millisecond,
		}
		cachedSecretManager = creds.NewCachedSecrets(secretManager, cacheConfig)
		secretParams = creds.SecretLookupParams{
			Team:     "some-team",
			Pipeline: "some-pipeline",
			Job:      "some-job",
		}
	})

	It("should handle missing secrets correctly and cache misses", func() {
		secretManager.GetStub, underlyingReads, underlyingMisses = makeGetStub("foo", "value", nil, true, nil)

		// miss
		value, expiration, found, err := cachedSecretManager.Get("bar", secretParams)
		Expect(value).To(BeNil())
		Expect(expiration).To(BeNil())
		Expect(found).To(BeFalse())
		Expect(err).To(BeNil())
		Expect(*underlyingReads).To(Equal(0))
		Expect(*underlyingMisses).To(Equal(1))

		// cached miss
		value, expiration, found, err = cachedSecretManager.Get("bar", secretParams)
		Expect(value).To(BeNil())
		Expect(expiration).To(BeNil())
		Expect(found).To(BeFalse())
		Expect(err).To(BeNil())
		Expect(*underlyingReads).To(Equal(0))
		Expect(*underlyingMisses).To(Equal(1))
	})

	It("should handle existing secrets correctly and cache them, returning previous value if the underlying value has changed", func() {
		secretManager.GetStub, underlyingReads, underlyingMisses = makeGetStub("foo", "value", nil, true, nil)

		// hit
		value, expiration, found, err := cachedSecretManager.Get("foo", secretParams)
		Expect(value).To(Equal("value"))
		Expect(expiration).To(BeNil())
		Expect(found).To(BeTrue())
		Expect(err).To(BeNil())
		Expect(*underlyingReads).To(Equal(1))
		Expect(*underlyingMisses).To(Equal(0))

		// cached hit
		secretManager.GetStub, underlyingReads, underlyingMisses = makeGetStub("foo", "different-value", nil, true, nil)
		value, expiration, found, err = cachedSecretManager.Get("foo", secretParams)
		Expect(value).To(Equal("value"), "should not return the newer value")
		Expect(expiration).To(BeNil())
		Expect(found).To(BeTrue())
		Expect(err).To(BeNil())
		Expect(*underlyingReads).To(Equal(0), "underlying secret manager should not be called")
		Expect(*underlyingMisses).To(Equal(0))
	})

	It("should handle errors correctly and avoid caching errors", func() {
		secretManager.GetStub, underlyingReads, underlyingMisses = makeGetStub("baz", nil, nil, false, fmt.Errorf("unexpected error"))

		// error
		value, expiration, found, err := cachedSecretManager.Get("baz", secretParams)
		Expect(value).To(BeNil())
		Expect(expiration).To(BeNil())
		Expect(found).To(BeFalse())
		Expect(err).NotTo(BeNil())
		Expect(*underlyingReads).To(Equal(1))
		Expect(*underlyingMisses).To(Equal(0))

		// no caching of error
		value, expiration, found, err = cachedSecretManager.Get("baz", secretParams)
		Expect(value).To(BeNil())
		Expect(expiration).To(BeNil())
		Expect(found).To(BeFalse())
		Expect(err).NotTo(BeNil())
		Expect(*underlyingReads).To(Equal(2))
		Expect(*underlyingMisses).To(Equal(0))
	})

	It("should re-retrieve expired entries", func() {
		secretManager.GetStub, underlyingReads, underlyingMisses = makeGetStub("foo", "value", nil, true, nil)

		// get few entries first
		_, _, _, _ = cachedSecretManager.Get("foo", secretParams)
		_, _, _, _ = cachedSecretManager.Get("bar", secretParams)
		_, _, _, _ = cachedSecretManager.Get("baz", secretParams)
		Expect(*underlyingReads).To(Equal(1))
		Expect(*underlyingMisses).To(Equal(2))

		// get these entries again and make sure they are cached
		_, _, _, _ = cachedSecretManager.Get("foo", secretParams)
		_, _, _, _ = cachedSecretManager.Get("bar", secretParams)
		_, _, _, _ = cachedSecretManager.Get("baz", secretParams)
		Expect(*underlyingReads).To(Equal(1))
		Expect(*underlyingMisses).To(Equal(2))

		// sleep
		time.Sleep(cacheConfig.Duration + time.Millisecond)

		// check counters again and make sure the entries are re-retrieved
		_, _, _, _ = cachedSecretManager.Get("foo", secretParams)
		_, _, _, _ = cachedSecretManager.Get("bar", secretParams)
		_, _, _, _ = cachedSecretManager.Get("baz", secretParams)
		Expect(*underlyingReads).To(Equal(2))
		Expect(*underlyingMisses).To(Equal(4))
	})

	It("should cache negative responses for a separately specified duration", func() {
		secretManager.GetStub, underlyingReads, underlyingMisses = makeGetStub("foo", "value", nil, true, nil)

		// get few entries first
		_, _, _, _ = cachedSecretManager.Get("foo", secretParams)
		_, _, _, _ = cachedSecretManager.Get("bar", secretParams)
		_, _, _, _ = cachedSecretManager.Get("baz", secretParams)
		Expect(*underlyingReads).To(Equal(1))
		Expect(*underlyingMisses).To(Equal(2))

		// sleep
		time.Sleep(cacheConfig.DurationNotFound + time.Millisecond)

		// existing secret should still be cached
		_, _, _, _ = cachedSecretManager.Get("foo", secretParams)
		Expect(*underlyingReads).To(Equal(1))
		Expect(*underlyingMisses).To(Equal(2))

		// non-existing secrets should be attempted to be retrieved again
		_, _, _, _ = cachedSecretManager.Get("bar", secretParams)
		_, _, _, _ = cachedSecretManager.Get("baz", secretParams)
		Expect(*underlyingReads).To(Equal(1))
		Expect(*underlyingMisses).To(Equal(4))
	})

	It("should not cache longer than default duration if durarion is 0 or less", func() {
		secretManager.GetStub, underlyingReads, underlyingMisses = makeGetStub("foo", "value", &time.Time{}, true, nil)

		// get few entries first
		_, _, _, _ = cachedSecretManager.Get("foo", secretParams)
		_, _, _, _ = cachedSecretManager.Get("bar", secretParams)
		_, _, _, _ = cachedSecretManager.Get("baz", secretParams)
		Expect(*underlyingReads).To(Equal(1))
		Expect(*underlyingMisses).To(Equal(2))

		// sleep
		time.Sleep(cacheConfig.Duration + time.Millisecond)

		// existing secret should be gone
		_, _, _, _ = cachedSecretManager.Get("foo", secretParams)
		Expect(*underlyingReads).To(Equal(2))
		Expect(*underlyingMisses).To(Equal(2))

		// non-existing secrets should be attempted to be retrieved again
		_, _, _, _ = cachedSecretManager.Get("bar", secretParams)
		_, _, _, _ = cachedSecretManager.Get("baz", secretParams)
		Expect(*underlyingReads).To(Equal(2))
		Expect(*underlyingMisses).To(Equal(4))
	})

})
