package lostandfound_test

import (
	"errors"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	dbfakes "github.com/concourse/atc/db/fakes"
	. "github.com/concourse/atc/lostandfound"
	"github.com/concourse/atc/lostandfound/fakes"
)

var _ = Describe("Runner", func() {
	var (
		fakeDB               *fakes.FakeRunnerDB
		fakeBaggageCollector *fakes.FakeBaggageCollector
		fakeClock            *fakeclock.FakeClock
		fakeLease            *dbfakes.FakeLease

		interval time.Duration

		process ifrit.Process
	)

	BeforeEach(func() {
		fakeDB = new(fakes.FakeRunnerDB)
		fakeBaggageCollector = new(fakes.FakeBaggageCollector)
		fakeLease = new(dbfakes.FakeLease)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))

		interval = 100 * time.Millisecond
	})

	JustBeforeEach(func() {
		process = ginkgomon.Invoke(NewRunner(
			lagertest.NewTestLogger("test"),
			fakeBaggageCollector,
			fakeDB,
			fakeClock,
			interval,
		))
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Expect(<-process.Wait()).ToNot(HaveOccurred())
	})

	Context("when the interval elapses", func() {
		JustBeforeEach(func() {
			fakeClock.WaitForWatcherAndIncrement(interval)
		})

		It("calls to get a lease for cache invalidation", func() {
			Eventually(fakeDB.LeaseCacheInvalidationCallCount).Should(Equal(1))
			_, actualInterval := fakeDB.LeaseCacheInvalidationArgsForCall(0)
			Expect(actualInterval).To(Equal(interval))
		})

		Context("when getting a lease succeeds", func() {
			BeforeEach(func() {
				fakeDB.LeaseCacheInvalidationReturns(fakeLease, true, nil)
			})

			It("it collects lost baggage", func() {
				Eventually(fakeBaggageCollector.CollectCallCount).Should(Equal(1))
			})

			It("breaks the lease", func() {
				Eventually(fakeLease.BreakCallCount).Should(Equal(1))
			})

			Context("when collecting fails", func() {
				BeforeEach(func() {
					fakeBaggageCollector.CollectReturns(errors.New("disaster"))
				})

				It("does not exit the process", func() {
					Consistently(process.Wait()).ShouldNot(Receive())
				})

				It("breaks the lease", func() {
					Eventually(fakeLease.BreakCallCount).Should(Equal(1))
				})
			})
		})

		Context("when getting a lease fails", func() {
			Context("because of an error", func() {
				BeforeEach(func() {
					fakeDB.LeaseCacheInvalidationReturns(nil, true, errors.New("disaster"))
				})

				It("does not exit and does not collect baggage", func() {
					Consistently(fakeBaggageCollector.CollectCallCount).Should(Equal(0))
					Consistently(process.Wait()).ShouldNot(Receive())
				})
			})

			Context("because we got leased of false", func() {
				BeforeEach(func() {
					fakeDB.LeaseCacheInvalidationReturns(nil, false, nil)
				})

				It("does not exit and does not collect baggage", func() {
					Consistently(fakeBaggageCollector.CollectCallCount).Should(Equal(0))
					Consistently(process.Wait()).ShouldNot(Receive())
				})
			})
		})
	})
})
