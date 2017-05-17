package resource_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/resource/resourcefakes"
	"github.com/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Fetcher", func() {
	var (
		fakeFetchSourceProvider *resourcefakes.FakeFetchSourceProvider
		fakeClock               *fakeclock.FakeClock
		fakeLockDB              *resourcefakes.FakeLockDB
		fetcher                 resource.Fetcher
		signals                 chan os.Signal
		ready                   chan struct{}
		resourceOptions         *resourcefakes.FakeResourceOptions
		fakeVersionedSource     *resourcefakes.FakeVersionedSource

		versionedSource resource.VersionedSource
		fetchErr        error
		teamID          = 123
	)

	BeforeEach(func() {
		fakeFetchSourceProviderFactory := new(resourcefakes.FakeFetchSourceProviderFactory)
		fakeFetchSourceProvider = new(resourcefakes.FakeFetchSourceProvider)
		fakeFetchSourceProviderFactory.NewFetchSourceProviderReturns(fakeFetchSourceProvider)

		fakeClock = fakeclock.NewFakeClock(time.Unix(0, 123))
		fakeLockDB = new(resourcefakes.FakeLockDB)

		fetcher = resource.NewFetcher(
			fakeClock,
			fakeLockDB,
			fakeFetchSourceProviderFactory,
		)

		signals = make(chan os.Signal)
		ready = make(chan struct{})
		resourceOptions = new(resourcefakes.FakeResourceOptions)
		fakeVersionedSource = new(resourcefakes.FakeVersionedSource)
	})

	JustBeforeEach(func() {
		versionedSource, fetchErr = fetcher.Fetch(
			lagertest.NewTestLogger("test"),
			resource.Session{},
			atc.Tags{},
			teamID,
			atc.VersionedResourceTypes{},
			new(resourcefakes.FakeResourceInstance),
			resource.EmptyMetadata{},
			new(workerfakes.FakeImageFetchingDelegate),
			resourceOptions,
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

		Context("when initialized", func() {
			BeforeEach(func() {
				fakeFetchSource.FindInitializedReturns(fakeVersionedSource, true, nil)
			})

			It("returns the source", func() {
				Expect(versionedSource).To(Equal(fakeVersionedSource))
			})

			It("closes the ready channel", func() {
				Expect(ready).To(BeClosed())
			})

			Context("when ioConfig has stdout", func() {
				var stdoutBuf *gbytes.Buffer

				BeforeEach(func() {
					stdoutBuf = gbytes.NewBuffer()
					resourceOptions.IOConfigReturns(resource.IOConfig{
						Stdout: stdoutBuf,
					})
				})

				It("logs helpful message", func() {
					Expect(stdoutBuf).To(gbytes.Say("using version of resource found in cache\n"))
				})
			})
		})

		Context("when not initialized", func() {
			BeforeEach(func() {
				fakeFetchSource.FindInitializedReturns(nil, false, nil)
				fakeFetchSource.LockNameReturns("fake-lock-name", nil)
			})

			Describe("failing to get a lock", func() {
				Context("when did not get a lock", func() {
					BeforeEach(func() {
						fakeLock := new(lockfakes.FakeLock)
						callCount := 0
						fakeLockDB.GetTaskLockStub = func(lager.Logger, string) (lock.Lock, bool, error) {
							callCount++
							fakeClock.Increment(resource.GetResourceLockInterval)
							if callCount == 1 {
								return nil, false, nil
							}
							return fakeLock, true, nil
						}
					})

					It("retries until it gets the lock", func() {
						Expect(fakeLockDB.GetTaskLockCallCount()).To(Equal(2))
					})

					It("initializes fetch source after lock is acquired", func() {
						Expect(fakeFetchSource.InitializeCallCount()).To(Equal(1))
					})
				})

				Context("when acquiring lock returns error", func() {
					BeforeEach(func() {
						fakeLock := new(lockfakes.FakeLock)
						callCount := 0
						fakeLockDB.GetTaskLockStub = func(lager.Logger, string) (lock.Lock, bool, error) {
							callCount++
							fakeClock.Increment(resource.GetResourceLockInterval)
							if callCount == 1 {
								return nil, false, errors.New("disaster")
							}
							return fakeLock, true, nil
						}
					})

					It("retries until it gets the lock", func() {
						Expect(fakeLockDB.GetTaskLockCallCount()).To(Equal(2))
					})

					It("initializes fetch source after lock is acquired", func() {
						Expect(fakeFetchSource.InitializeCallCount()).To(Equal(1))
					})
				})
			})

			Context("when getting lock succeeds", func() {
				var fakeLock *lockfakes.FakeLock

				BeforeEach(func() {
					fakeLock = new(lockfakes.FakeLock)
					fakeLockDB.GetTaskLockReturns(fakeLock, true, nil)
					fakeFetchSource.InitializeReturns(fakeVersionedSource, nil)
				})

				It("acquires a lock with source lock name", func() {
					Expect(fakeLockDB.GetTaskLockCallCount()).To(Equal(1))
					_, lockName := fakeLockDB.GetTaskLockArgsForCall(0)
					Expect(lockName).To(Equal("fake-lock-name"))
				})

				It("releases the lock", func() {
					Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
				})

				It("initializes source", func() {
					Expect(fakeFetchSource.InitializeCallCount()).To(Equal(1))
				})

				It("returns the source", func() {
					Expect(versionedSource).To(Equal(fakeVersionedSource))
				})
			})
		})

		Context("when checking if initialized fails", func() {
			var disaster error

			BeforeEach(func() {
				disaster = errors.New("fail")
				fakeFetchSource.FindInitializedReturns(nil, false, disaster)
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
