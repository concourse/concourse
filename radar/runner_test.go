package radar_test

import (
	"errors"
	"os"

	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/radar/fakes"
	"github.com/concourse/turbine"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner", func() {
	var (
		locker          *fakes.FakeLocker
		scanner         *fakes.FakeScanner
		noop            bool
		resources       config.Resources
		turbineEndpoint *rata.RequestGenerator

		lock *dbfakes.FakeLock

		process ifrit.Process
	)

	BeforeEach(func() {
		locker = new(fakes.FakeLocker)
		scanner = new(fakes.FakeScanner)

		noop = false

		resources = config.Resources{
			{
				Name: "some-resource",
			},
			{
				Name: "some-other-resource",
			},
		}

		turbineEndpoint = rata.NewRequestGenerator("turbine-host", turbine.Routes)

		lock = new(dbfakes.FakeLock)
		locker.AcquireResourceCheckingLockReturns(lock, nil)
	})

	JustBeforeEach(func() {
		process = ginkgomon.Invoke(&Runner{
			Locker:          locker,
			Scanner:         scanner,
			Noop:            noop,
			Resources:       resources,
			TurbineEndpoint: turbineEndpoint,
		})
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	It("acquires the resource checking lock", func() {
		Eventually(locker.AcquireResourceCheckingLockCallCount).Should(Equal(1))
	})

	It("scans for every given resource", func() {
		Eventually(scanner.ScanCallCount).Should(Equal(2))

		_, resource := scanner.ScanArgsForCall(0)
		Ω(resource).Should(Equal(config.Resource{Name: "some-resource"}))

		_, resource = scanner.ScanArgsForCall(1)
		Ω(resource).Should(Equal(config.Resource{Name: "some-other-resource"}))
	})

	Context("when the lock cannot be acquired immediately", func() {
		var acquiredLocks chan<- db.Lock

		BeforeEach(func() {
			locks := make(chan db.Lock)
			acquiredLocks = locks

			locker.AcquireResourceCheckingLockStub = func() (db.Lock, error) {
				return <-locks, nil
			}
		})

		It("starts immediately regardless", func() {})

		Context("when told to stop", func() {
			JustBeforeEach(func() {
				process.Signal(os.Interrupt)
			})

			It("exits regardless", func() {
				Eventually(process.Wait()).Should(Receive())
			})
		})
	})

	Context("when told to stop", func() {
		JustBeforeEach(func() {
			// ensure that we've acquired the lock
			Eventually(scanner.ScanCallCount).ShouldNot(BeZero())

			process.Signal(os.Interrupt)
		})

		It("releases the resource checking lock", func() {
			Eventually(lock.ReleaseCallCount).Should(Equal(1))
		})

		Context("and releasing the lock fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				lock.ReleaseReturns(disaster)
			})

			It("returns the error", func() {
				Eventually(process.Wait()).Should(Receive(Equal(disaster)))
			})
		})
	})

	Context("when in noop mode", func() {
		BeforeEach(func() {
			noop = true
		})

		It("does not acquire the lock", func() {
			Ω(locker.AcquireResourceCheckingLockCallCount()).Should(Equal(0))
		})

		It("does not start scanning resources", func() {
			Ω(scanner.ScanCallCount()).Should(Equal(0))
		})
	})
})
