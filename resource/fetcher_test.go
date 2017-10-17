package resource_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/resource/resourcefakes"
	"github.com/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fetcher", func() {
	var (
		fakeFetchSourceProvider *resourcefakes.FakeFetchSourceProvider
		fakeClock               *fakeclock.FakeClock
		fakeLockFactory         *lockfakes.FakeLockFactory
		fetcher                 resource.Fetcher
		signals                 chan os.Signal
		ready                   chan struct{}
		fakeVersionedSource     *resourcefakes.FakeVersionedSource
		fakeBuildStepDelegate   *workerfakes.FakeImageFetchingDelegate

		versionedSource resource.VersionedSource
		fetchErr        error
		teamID          = 123
	)

	BeforeEach(func() {
		fakeFetchSourceProviderFactory := new(resourcefakes.FakeFetchSourceProviderFactory)
		fakeFetchSourceProvider = new(resourcefakes.FakeFetchSourceProvider)
		fakeFetchSourceProviderFactory.NewFetchSourceProviderReturns(fakeFetchSourceProvider)

		fakeClock = fakeclock.NewFakeClock(time.Unix(0, 123))
		fakeLockFactory = new(lockfakes.FakeLockFactory)

		fetcher = resource.NewFetcher(
			fakeClock,
			fakeLockFactory,
			fakeFetchSourceProviderFactory,
		)

		signals = make(chan os.Signal)
		ready = make(chan struct{})
		fakeVersionedSource = new(resourcefakes.FakeVersionedSource)

		fakeBuildStepDelegate = new(workerfakes.FakeImageFetchingDelegate)
	})

	JustBeforeEach(func() {
		versionedSource, fetchErr = fetcher.Fetch(
			lagertest.NewTestLogger("test"),
			resource.Session{},
			atc.Tags{},
			teamID,
			creds.VersionedResourceTypes{},
			new(resourcefakes.FakeResourceInstance),
			resource.EmptyMetadata{},
			fakeBuildStepDelegate,
			signals,
			ready,
		)
	})

	Context("when getting source succeeds", func() {
		var fakeFetchSource *resourcefakes.FakeFetchSource

		BeforeEach(func() {
			fakeFetchSource = new(resourcefakes.FakeFetchSource)
			fakeFetchSourceProvider.GetReturns(fakeFetchSource, nil)
		})

		Context("when found", func() {
			BeforeEach(func() {
				fakeFetchSource.FindReturns(fakeVersionedSource, true, nil)
			})

			It("returns the source", func() {
				Expect(versionedSource).To(Equal(fakeVersionedSource))
			})

			It("closes the ready channel", func() {
				Expect(ready).To(BeClosed())
			})
		})

		Context("when not found", func() {
			BeforeEach(func() {
				fakeFetchSource.FindReturns(nil, false, nil)
				fakeFetchSource.LockNameReturns("fake-lock-name", nil)
			})

			Describe("failing to get a lock", func() {
				Context("when did not get a lock", func() {
					BeforeEach(func() {
						fakeLock := new(lockfakes.FakeLock)
						callCount := 0
						fakeLockFactory.AcquireStub = func(lager.Logger, lock.LockID) (lock.Lock, bool, error) {
							callCount++
							fakeClock.Increment(resource.GetResourceLockInterval)
							if callCount == 1 {
								return nil, false, nil
							}
							return fakeLock, true, nil
						}
					})

					It("retries until it gets the lock", func() {
						Expect(fakeLockFactory.AcquireCallCount()).To(Equal(2))
					})

					It("creates fetch source after lock is acquired", func() {
						Expect(fakeFetchSource.CreateCallCount()).To(Equal(1))
					})
				})

				Context("when acquiring lock returns error", func() {
					BeforeEach(func() {
						fakeLock := new(lockfakes.FakeLock)
						callCount := 0
						fakeLockFactory.AcquireStub = func(lager.Logger, lock.LockID) (lock.Lock, bool, error) {
							callCount++
							fakeClock.Increment(resource.GetResourceLockInterval)
							if callCount == 1 {
								return nil, false, errors.New("disaster")
							}
							return fakeLock, true, nil
						}
					})

					It("retries until it gets the lock", func() {
						Expect(fakeLockFactory.AcquireCallCount()).To(Equal(2))
					})

					It("creates fetch source after lock is acquired", func() {
						Expect(fakeFetchSource.CreateCallCount()).To(Equal(1))
					})
				})
			})

			Context("when getting lock succeeds", func() {
				var fakeLock *lockfakes.FakeLock

				BeforeEach(func() {
					fakeLock = new(lockfakes.FakeLock)
					fakeLockFactory.AcquireReturns(fakeLock, true, nil)
					fakeFetchSource.CreateReturns(fakeVersionedSource, nil)
				})

				It("acquires a lock with source lock name", func() {
					Expect(fakeLockFactory.AcquireCallCount()).To(Equal(1))
					_, lockID := fakeLockFactory.AcquireArgsForCall(0)
					Expect(lockID).To(Equal(lock.NewTaskLockID("fake-lock-name")))
				})

				It("releases the lock", func() {
					Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
				})

				It("creates source", func() {
					Expect(fakeFetchSource.CreateCallCount()).To(Equal(1))
				})

				It("returns the source", func() {
					Expect(versionedSource).To(Equal(fakeVersionedSource))
				})
			})
		})

		Context("when finding fails", func() {
			var disaster error

			BeforeEach(func() {
				disaster = errors.New("fail")
				fakeFetchSource.FindReturns(nil, false, disaster)
			})

			It("returns an error", func() {
				Expect(fetchErr).To(HaveOccurred())
				Expect(fetchErr).To(Equal(disaster))
			})
		})

		Context("when signal is received", func() {
			BeforeEach(func() {
				go func() {
					signals <- os.Interrupt
				}()
			})

			It("returns ErrInterrupted", func() {
				Expect(fetchErr).To(Equal(resource.ErrInterrupted))
			})
		})
	})
})
