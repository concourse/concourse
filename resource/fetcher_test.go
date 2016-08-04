package resource_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/dbfakes"
	. "github.com/concourse/atc/resource"
	"github.com/concourse/atc/resource/resourcefakes"
	"github.com/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Fetcher", func() {
	var (
		fakeFetchContainerCreator *resourcefakes.FakeFetchContainerCreator
		fakeFetchSourceProvider   *resourcefakes.FakeFetchSourceProvider
		fakeClock                 *fakeclock.FakeClock
		fakeLeaseDB               *resourcefakes.FakeLeaseDB
		fetcher                   Fetcher
		signals                   chan os.Signal
		ready                     chan struct{}
		resourceOptions           *resourcefakes.FakeResourceOptions

		fetchSource FetchSource
		fetchErr    error
		teamID      = 123
	)

	BeforeEach(func() {
		fakeFetchContainerCreatorFactory := new(resourcefakes.FakeFetchContainerCreatorFactory)
		fakeFetchContainerCreator = new(resourcefakes.FakeFetchContainerCreator)
		fakeFetchContainerCreatorFactory.NewFetchContainerCreatorReturns(fakeFetchContainerCreator)

		fakeFetchSourceProviderFactory := new(resourcefakes.FakeFetchSourceProviderFactory)
		fakeFetchSourceProvider = new(resourcefakes.FakeFetchSourceProvider)
		fakeFetchSourceProviderFactory.NewFetchSourceProviderReturns(fakeFetchSourceProvider)

		fakeClock = fakeclock.NewFakeClock(time.Unix(0, 123))
		fakeLeaseDB = new(resourcefakes.FakeLeaseDB)

		fetcher = NewFetcher(
			fakeClock,
			fakeLeaseDB,
			fakeFetchContainerCreatorFactory,
			fakeFetchSourceProviderFactory,
		)

		signals = make(chan os.Signal)
		ready = make(chan struct{})
		resourceOptions = new(resourcefakes.FakeResourceOptions)
	})

	JustBeforeEach(func() {
		fetchSource, fetchErr = fetcher.Fetch(
			lagertest.NewTestLogger("test"),
			Session{},
			atc.Tags{},
			teamID,
			atc.ResourceTypes{},
			new(resourcefakes.FakeCacheIdentifier),
			EmptyMetadata{},
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
				fakeFetchSource.IsInitializedReturns(true, nil)
			})

			It("returns the source", func() {
				Expect(fetchSource).To(Equal(fakeFetchSource))
			})

			It("closes the ready channel", func() {
				Expect(ready).To(BeClosed())
			})

			Context("when ioConfig has stdout", func() {
				var stdoutBuf *gbytes.Buffer

				BeforeEach(func() {
					stdoutBuf = gbytes.NewBuffer()
					resourceOptions.IOConfigReturns(IOConfig{
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
				fakeFetchSource.IsInitializedReturns(false, nil)
				fakeFetchSource.LeaseNameReturns("fake-lease-name", nil)
			})

			Describe("failing to get a lease", func() {
				BeforeEach(func() {
					callCount := 0
					fakeFetchSource.IsInitializedStub = func() (bool, error) {
						callCount++
						fakeClock.Increment(GetResourceLeaseInterval)
						if callCount == 1 {
							return false, nil
						}

						return true, nil
					}
				})

				Context("when did not get a lease", func() {
					BeforeEach(func() {
						fakeLeaseDB.GetLeaseReturns(nil, false, nil)
					})

					It("does not initialize fetch source", func() {
						Expect(fakeFetchSource.InitializeCallCount()).To(Equal(0))
					})

					It("retries until it gets initialized source", func() {
						Expect(fakeFetchSourceProvider.GetCallCount()).To(Equal(2))
					})
				})

				Context("when acquiring lease returns error", func() {
					BeforeEach(func() {
						fakeLeaseDB.GetLeaseReturns(nil, false, errors.New("disaster"))
					})

					It("does not initialize fetch source", func() {
						Expect(fakeFetchSource.InitializeCallCount()).To(Equal(0))
					})

					It("retries until it gets initialized source", func() {
						Expect(fakeFetchSourceProvider.GetCallCount()).To(Equal(2))
					})
				})
			})

			Context("when getting lease succeeds", func() {
				var fakeLease *dbfakes.FakeLease

				BeforeEach(func() {
					fakeLease = new(dbfakes.FakeLease)
					fakeLeaseDB.GetLeaseReturns(fakeLease, true, nil)
				})

				It("acquires a lease with source lease name", func() {
					Expect(fakeLeaseDB.GetLeaseCallCount()).To(Equal(1))
					_, leaseName, interval := fakeLeaseDB.GetLeaseArgsForCall(0)
					Expect(leaseName).To(Equal("fake-lease-name"))
					Expect(interval).To(Equal(GetResourceLeaseInterval))
				})

				It("releases the lease", func() {
					Expect(fakeLease.BreakCallCount()).To(Equal(1))
				})

				It("initializes source", func() {
					Expect(fakeFetchSource.InitializeCallCount()).To(Equal(1))
				})

				It("returns the source", func() {
					Expect(fetchSource).To(Equal(fakeFetchSource))
				})
			})
		})

		Context("when checking if initialized fails", func() {
			var disaster error

			BeforeEach(func() {
				disaster = errors.New("fail")
				fakeFetchSource.IsInitializedReturns(false, disaster)
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
				Expect(fetchErr).To(Equal(ErrInterrupted))
			})
		})
	})
})
