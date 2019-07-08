package resource_test

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fetcher", func() {
	var (
		fakeClock             *fakeclock.FakeClock
		fakeLockFactory       *lockfakes.FakeLockFactory
		fetcher               resource.Fetcher
		ctx                   context.Context
		cancel                func()
		fakeVersionedSource   *resourcefakes.FakeVersionedSource
		fakeBuildStepDelegate *workerfakes.FakeImageFetchingDelegate

		fakeWorker             *workerfakes.FakeWorker
		fakeFetchSourceFactory *resourcefakes.FakeFetchSourceFactory

		versionedSource resource.VersionedSource
		fetchErr        error
		teamID          = 123
	)

	BeforeEach(func() {
		fakeClock = fakeclock.NewFakeClock(time.Unix(0, 123))
		fakeLockFactory = new(lockfakes.FakeLockFactory)
		fakeFetchSourceFactory = new(resourcefakes.FakeFetchSourceFactory)

		fakeWorker = new(workerfakes.FakeWorker)
		fakeWorker.NameReturns("some-worker")

		fetcher = resource.NewFetcher(
			fakeClock,
			fakeLockFactory,
			fakeFetchSourceFactory,
		)

		ctx, cancel = context.WithCancel(context.Background())
		fakeVersionedSource = new(resourcefakes.FakeVersionedSource)

		fakeBuildStepDelegate = new(workerfakes.FakeImageFetchingDelegate)
	})

	JustBeforeEach(func() {
		versionedSource, fetchErr = fetcher.Fetch(
			ctx,
			lagertest.NewTestLogger("test"),
			resource.Session{},
			fakeWorker,
			worker.ContainerSpec{
				TeamID: teamID,
			},
			atc.VersionedResourceTypes{},
			new(resourcefakes.FakeResourceInstance),
			fakeBuildStepDelegate,
			nil,
		)
	})

	Context("when getting source", func() {
		var fakeFetchSource *resourcefakes.FakeFetchSource

		BeforeEach(func() {
			fakeFetchSource = new(resourcefakes.FakeFetchSource)
			fakeFetchSourceFactory.NewFetchSourceReturns(fakeFetchSource)

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

		Context("when cancelled", func() {
			BeforeEach(func() {
				cancel()
			})

			It("returns the context err", func() {
				Expect(fetchErr).To(Equal(context.Canceled))
			})
		})
	})
})
