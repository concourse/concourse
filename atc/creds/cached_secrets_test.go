package creds_test

import (
	"fmt"
	"time"

	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/vars"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func makeGetStub(name string, value interface{}, expiration *time.Time, found bool, err error, cntReads *int, cntMisses *int) func(vars.VariableReference) (interface{}, *time.Time, bool, error) {
	return func(ref vars.VariableReference) (interface{}, *time.Time, bool, error) {
		if ref.Name == name {
			*cntReads++
			return value, expiration, found, err
		}
		*cntMisses++
		return nil, nil, false, nil
	}
}

var _ = Describe("Caching of secrets", func() {

	var secretManager *credsfakes.FakeSecrets
	var cacheConfig creds.SecretCacheConfig
	var cachedSecretManager *creds.CachedSecrets
	var underlyingReads int
	var underlyingMisses int

	BeforeEach(func() {
		secretManager = new(credsfakes.FakeSecrets)
		cacheConfig = creds.SecretCacheConfig{
			Duration:         2 * time.Second,
			DurationNotFound: 1 * time.Second,
			PurgeInterval:    100 * time.Millisecond,
		}
		cachedSecretManager = creds.NewCachedSecrets(secretManager, cacheConfig)
		underlyingReads = 0
		underlyingMisses = 0
	})

	It("should handle missing secrets correctly and cache misses", func() {
		secretManager.GetStub = makeGetStub("foo", "value", nil, true, nil, &underlyingReads, &underlyingMisses)

		// miss
		value, expiration, found, err := cachedSecretManager.Get(vars.VariableReference{Name: "bar"})
		Expect(value).To(BeNil())
		Expect(expiration).To(BeNil())
		Expect(found).To(BeFalse())
		Expect(err).To(BeNil())
		Expect(underlyingReads).To(BeIdenticalTo(0))
		Expect(underlyingMisses).To(BeIdenticalTo(1))

		// cached miss
		value, expiration, found, err = cachedSecretManager.Get(vars.VariableReference{Name: "bar"})
		Expect(value).To(BeNil())
		Expect(expiration).To(BeNil())
		Expect(found).To(BeFalse())
		Expect(err).To(BeNil())
		Expect(underlyingReads).To(BeIdenticalTo(0))
		Expect(underlyingMisses).To(BeIdenticalTo(1))
	})

	It("should handle existing secrets correctly and cache them, returning previous value if the underlying value has changed", func() {
		secretManager.GetStub = makeGetStub("foo", "value", nil, true, nil, &underlyingReads, &underlyingMisses)

		// hit
		value, expiration, found, err := cachedSecretManager.Get(vars.VariableReference{Name: "foo"})
		Expect(value).To(BeIdenticalTo("value"))
		Expect(expiration).To(BeNil())
		Expect(found).To(BeTrue())
		Expect(err).To(BeNil())
		Expect(underlyingReads).To(BeIdenticalTo(1))
		Expect(underlyingMisses).To(BeIdenticalTo(0))

		// cached hit
		secretManager.GetStub = makeGetStub("foo", "different-value", nil, true, nil, &underlyingReads, &underlyingMisses)
		value, expiration, found, err = cachedSecretManager.Get(vars.VariableReference{Name: "foo"})
		Expect(value).To(BeIdenticalTo("value"))
		Expect(expiration).To(BeNil())
		Expect(found).To(BeTrue())
		Expect(err).To(BeNil())
		Expect(underlyingReads).To(BeIdenticalTo(1))
		Expect(underlyingMisses).To(BeIdenticalTo(0))
	})

	It("should handle errors correctly and avoid caching errors", func() {
		secretManager.GetStub = makeGetStub("baz", nil, nil, false, fmt.Errorf("unexpected error"), &underlyingReads, &underlyingMisses)

		// error
		value, expiration, found, err := cachedSecretManager.Get(vars.VariableReference{Name: "baz"})
		Expect(value).To(BeNil())
		Expect(expiration).To(BeNil())
		Expect(found).To(BeFalse())
		Expect(err).NotTo(BeNil())
		Expect(underlyingReads).To(BeIdenticalTo(1))
		Expect(underlyingMisses).To(BeIdenticalTo(0))

		// no caching of error
		value, expiration, found, err = cachedSecretManager.Get(vars.VariableReference{Name: "baz"})
		Expect(value).To(BeNil())
		Expect(expiration).To(BeNil())
		Expect(found).To(BeFalse())
		Expect(err).NotTo(BeNil())
		Expect(underlyingReads).To(BeIdenticalTo(2))
		Expect(underlyingMisses).To(BeIdenticalTo(0))
	})

	It("should re-retrieve expired entries", func() {
		secretManager.GetStub = makeGetStub("foo", "value", nil, true, nil, &underlyingReads, &underlyingMisses)

		// get few entries first
		_, _, _, _ = cachedSecretManager.Get(vars.VariableReference{Name: "foo"})
		_, _, _, _ = cachedSecretManager.Get(vars.VariableReference{Name: "bar"})
		_, _, _, _ = cachedSecretManager.Get(vars.VariableReference{Name: "baz"})
		Expect(underlyingReads).To(BeIdenticalTo(1))
		Expect(underlyingMisses).To(BeIdenticalTo(2))

		// get these entries again and make sure they are cached
		_, _, _, _ = cachedSecretManager.Get(vars.VariableReference{Name: "foo"})
		_, _, _, _ = cachedSecretManager.Get(vars.VariableReference{Name: "bar"})
		_, _, _, _ = cachedSecretManager.Get(vars.VariableReference{Name: "baz"})
		Expect(underlyingReads).To(BeIdenticalTo(1))
		Expect(underlyingMisses).To(BeIdenticalTo(2))

		// sleep
		time.Sleep(cacheConfig.Duration + time.Millisecond)

		// check counters again and make sure the entries are re-retrieved
		_, _, _, _ = cachedSecretManager.Get(vars.VariableReference{Name: "foo"})
		_, _, _, _ = cachedSecretManager.Get(vars.VariableReference{Name: "bar"})
		_, _, _, _ = cachedSecretManager.Get(vars.VariableReference{Name: "baz"})
		Expect(underlyingReads).To(BeIdenticalTo(2))
		Expect(underlyingMisses).To(BeIdenticalTo(4))
	})

	It("should cache negative responses for a separately specified duration", func() {
		secretManager.GetStub = makeGetStub("foo", "value", nil, true, nil, &underlyingReads, &underlyingMisses)

		// get few entries first
		_, _, _, _ = cachedSecretManager.Get(vars.VariableReference{Name: "foo"})
		_, _, _, _ = cachedSecretManager.Get(vars.VariableReference{Name: "bar"})
		_, _, _, _ = cachedSecretManager.Get(vars.VariableReference{Name: "baz"})
		Expect(underlyingReads).To(BeIdenticalTo(1))
		Expect(underlyingMisses).To(BeIdenticalTo(2))

		// sleep
		time.Sleep(cacheConfig.DurationNotFound + time.Millisecond)

		// existing secret should still be cached
		_, _, _, _ = cachedSecretManager.Get(vars.VariableReference{Name: "foo"})
		Expect(underlyingReads).To(BeIdenticalTo(1))
		Expect(underlyingMisses).To(BeIdenticalTo(2))

		// non-existing secrets should be attempted to be retrieved again
		_, _, _, _ = cachedSecretManager.Get(vars.VariableReference{Name: "bar"})
		_, _, _, _ = cachedSecretManager.Get(vars.VariableReference{Name: "baz"})
		Expect(underlyingReads).To(BeIdenticalTo(1))
		Expect(underlyingMisses).To(BeIdenticalTo(4))
	})

})
