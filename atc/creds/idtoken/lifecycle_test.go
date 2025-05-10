package idtoken_test

import (
	"fmt"
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

	var signingKeyFactory db.SigningKeyFactory
	var lockFactory lock.LockFactory

	BeforeEach(func() {
		signingKeys := make([]db.SigningKey, 0, 1)

		signingKeyFactoryFake := &dbfakes.FakeSigningKeyFactory{}
		signingKeyFactoryFake.CreateKeyStub = func(jwk jose.JSONWebKey) error {
			key := &dbfakes.FakeSigningKey{}
			key.JWKReturns(jwk)
			key.IDReturns(jwk.KeyID)
			key.CreatedAtReturns(time.Now())
			key.KeyTypeReturns(db.SigningKeyTypeRSA)

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
	})

	It("makes sure one suitable signing key exists", func() {
		before, err := signingKeyFactory.GetAllKeys()
		Expect(err).ToNot(HaveOccurred())
		Expect(before).To(HaveLen(0))

		idtoken.EnsureSigningKeysExist(lager.NewLogger(""), signingKeyFactory, lockFactory)

		after, err := signingKeyFactory.GetAllKeys()
		Expect(err).ToNot(HaveOccurred())
		Expect(after).To(HaveLen(1))

		kid := after[0].ID()

		// make sure a re-run does not create an additional key
		idtoken.EnsureSigningKeysExist(lager.NewLogger(""), signingKeyFactory, lockFactory)
		after, err = signingKeyFactory.GetAllKeys()
		Expect(err).ToNot(HaveOccurred())
		Expect(after).To(HaveLen(1))
		Expect(after[0].ID()).To(Equal(kid))

	})

})
