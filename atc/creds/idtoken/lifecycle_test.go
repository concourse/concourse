package idtoken_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/creds/idtoken"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/go-jose/go-jose/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IDToken Lifecycle", func() {

	var signingKeys []db.SigningKey
	var signingKeyFactory db.SigningKeyFactory
	var lockFactory lock.LockFactory
	var lifecycler idtoken.SigningKeyLifecycler
	var ctx context.Context

	BeforeEach(func() {
		signingKeys = make([]db.SigningKey, 0, 4)

		signingKeyFactoryFake := &dbfakes.FakeSigningKeyFactory{}
		signingKeyFactoryFake.CreateKeyStub = func(jwk jose.JSONWebKey) error {
			key := createFakeSigningKey(jwk, time.Now())
			signingKeys = append(signingKeys, key)

			return nil
		}

		signingKeyFactoryFake.GetAllKeysStub = func() ([]db.SigningKey, error) {
			return signingKeys, nil
		}

		signingKeyFactoryFake.GetNewestKeyStub = func(kty db.SigningKeyType) (db.SigningKey, error) {
			var newest db.SigningKey
			for _, key := range signingKeys {
				if key.KeyType() == kty {
					if newest == nil || newest.CreatedAt().Before(key.CreatedAt()) {
						newest = key
					}
				}
			}
			if newest != nil {
				return newest, nil
			}
			return nil, fmt.Errorf("not found")
		}

		signingKeyFactory = signingKeyFactoryFake

		fakeLockFactory := &lockfakes.FakeLockFactory{}
		fakeLockFactory.AcquireStub = func(l lager.Logger, li lock.LockID) (lock.Lock, bool, error) {
			return new(lockfakes.FakeLock), true, nil
		}
		lockFactory = fakeLockFactory

		lifecycler = idtoken.SigningKeyLifecycler{
			Logger:              lager.NewLogger(""),
			DBSigningKeyFactory: signingKeyFactory,
			LockFactory:         lockFactory,

			CheckPeriod:       10 * time.Second,
			KeyRotationPeriod: 1 * time.Hour,
			KeyGracePeriod:    10 * time.Minute,
		}

		ctx = context.Background()
	})

	It("makes sure signing keys are created when none exist", func() {
		before, err := signingKeyFactory.GetAllKeys()
		Expect(err).ToNot(HaveOccurred())
		Expect(before).To(HaveLen(0))

		Expect(lifecycler.RunOnce(ctx)).To(Succeed())

		after, err := signingKeyFactory.GetAllKeys()
		Expect(err).ToNot(HaveOccurred())
		Expect(after).To(HaveLen(2))

		rsaKey, err := signingKeyFactory.GetNewestKey(db.SigningKeyTypeRSA)
		Expect(err).ToNot(HaveOccurred())
		Expect(rsaKey.KeyType()).To(Equal(db.SigningKeyTypeRSA))

		ecKey, err := signingKeyFactory.GetNewestKey(db.SigningKeyTypeEC)
		Expect(err).ToNot(HaveOccurred())
		Expect(ecKey.KeyType()).To(Equal(db.SigningKeyTypeEC))

		// make sure a re-run does not create additional keys
		Expect(lifecycler.RunOnce(ctx)).To(Succeed())
		after, err = signingKeyFactory.GetAllKeys()
		Expect(err).ToNot(HaveOccurred())
		Expect(after).To(HaveLen(2))
	})

	It("generates new keys when existing keys are too old", func() {
		oldRSAKey := createFakeSigningKey(*rsaJWK, time.Now().Add(-61*time.Minute))
		oldECKey := createFakeSigningKey(*ecJWK, time.Now().Add(-61*time.Minute))
		signingKeys = append(signingKeys, oldRSAKey, oldECKey)

		before, err := signingKeyFactory.GetAllKeys()
		Expect(err).ToNot(HaveOccurred())
		Expect(before).To(HaveLen(2))

		Expect(lifecycler.RunOnce(ctx)).To(Succeed())

		after, err := signingKeyFactory.GetAllKeys()
		Expect(err).ToNot(HaveOccurred())
		// old keys are not deleted until after the grace period, so we should now have 4 keys
		Expect(after).To(HaveLen(4))
		Expect(oldRSAKey.DeleteCallCount()).To(Equal(0))
		Expect(oldECKey.DeleteCallCount()).To(Equal(0))

		rsaKey, err := signingKeyFactory.GetNewestKey(db.SigningKeyTypeRSA)
		Expect(err).ToNot(HaveOccurred())
		Expect(rsaKey.KeyType()).To(Equal(db.SigningKeyTypeRSA))
		Expect(rsaKey.ID()).NotTo(Equal(oldRSAKey.ID()))

		ecKey, err := signingKeyFactory.GetNewestKey(db.SigningKeyTypeEC)
		Expect(err).ToNot(HaveOccurred())
		Expect(ecKey.KeyType()).To(Equal(db.SigningKeyTypeEC))
		Expect(ecKey.ID()).NotTo(Equal(oldECKey.ID()))

		// make sure a re-run does not create additional keys
		Expect(lifecycler.RunOnce(ctx)).To(Succeed())
		after, err = signingKeyFactory.GetAllKeys()
		Expect(err).ToNot(HaveOccurred())
		Expect(after).To(HaveLen(4))
	})

	It("removes outdated keys after grace period", func() {
		oldRSAKey := createFakeSigningKey(*rsaJWK, time.Now().Add(-3*time.Hour))
		oldECKey := createFakeSigningKey(*ecJWK, time.Now().Add(-3*time.Hour))
		newRSAKey := createFakeSigningKey(*rsaJWK, time.Now().Add(-12*time.Minute))
		newECKey := createFakeSigningKey(*ecJWK, time.Now().Add(-12*time.Minute))
		signingKeys = append(signingKeys, oldRSAKey, oldECKey, newRSAKey, newECKey)

		before, err := signingKeyFactory.GetAllKeys()
		Expect(err).ToNot(HaveOccurred())
		Expect(before).To(HaveLen(4))

		Expect(lifecycler.RunOnce(ctx)).To(Succeed())

		Expect(oldRSAKey.DeleteCallCount()).To(Equal(1))
		Expect(oldECKey.DeleteCallCount()).To(Equal(1))
		Expect(newRSAKey.DeleteCallCount()).To(Equal(0))
		Expect(newECKey.DeleteCallCount()).To(Equal(0))
	})
})

func createFakeSigningKey(jwk jose.JSONWebKey, createdAt time.Time) *dbfakes.FakeSigningKey {

	generateRandomNumericString := func() string {
		num, _ := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
		return strconv.Itoa(int(num.Int64()))
	}

	jwk.KeyID = generateRandomNumericString()

	key := &dbfakes.FakeSigningKey{}
	key.JWKReturns(jwk)
	key.IDReturns(jwk.KeyID)
	key.CreatedAtReturns(createdAt)

	if jwk.Algorithm == "RS256" {
		key.KeyTypeReturns(db.SigningKeyTypeRSA)
	} else if jwk.Algorithm == "ES256" {
		key.KeyTypeReturns(db.SigningKeyTypeEC)
	}
	return key
}
