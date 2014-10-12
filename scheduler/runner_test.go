package scheduler_test

import (
	"errors"
	"os"
	"time"

	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	. "github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/fakes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner", func() {
	var (
		locker    *fakes.FakeLocker
		scheduler *fakes.FakeBuildScheduler
		noop      bool
		jobs      config.Jobs

		lock *dbfakes.FakeLock

		process ifrit.Process
	)

	BeforeEach(func() {
		locker = new(fakes.FakeLocker)
		scheduler = new(fakes.FakeBuildScheduler)

		noop = false

		jobs = config.Jobs{
			{
				Name: "some-job",
			},
			{
				Name: "some-other-job",
			},
		}

		lock = new(dbfakes.FakeLock)
		locker.AcquireBuildSchedulingLockReturns(lock, nil)
	})

	JustBeforeEach(func() {
		process = ginkgomon.Invoke(&Runner{
			Locker:    locker,
			Scheduler: scheduler,
			Noop:      noop,
			Jobs:      jobs,
			Interval:  100 * time.Millisecond,
		})
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	It("acquires the build scheduling lock", func() {
		Eventually(locker.AcquireBuildSchedulingLockCallCount).Should(Equal(1))
	})

	It("schedules pending builds", func() {
		Eventually(scheduler.TryNextPendingBuildCallCount).Should(Equal(2))

		job := scheduler.TryNextPendingBuildArgsForCall(0)
		Ω(job).Should(Equal(config.Job{Name: "some-job"}))

		job = scheduler.TryNextPendingBuildArgsForCall(1)
		Ω(job).Should(Equal(config.Job{Name: "some-other-job"}))
	})

	It("schedules builds for new inputs", func() {
		Eventually(scheduler.BuildLatestInputsCallCount).Should(Equal(2))

		job := scheduler.BuildLatestInputsArgsForCall(0)
		Ω(job).Should(Equal(config.Job{Name: "some-job"}))

		job = scheduler.BuildLatestInputsArgsForCall(1)
		Ω(job).Should(Equal(config.Job{Name: "some-other-job"}))
	})

	Context("when the lock cannot be acquired immediately", func() {
		var acquiredLocks chan<- db.Lock

		BeforeEach(func() {
			locks := make(chan db.Lock)
			acquiredLocks = locks

			locker.AcquireBuildSchedulingLockStub = func() (db.Lock, error) {
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
			Eventually(scheduler.TryNextPendingBuildCallCount).ShouldNot(BeZero())

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
			Consistently(locker.AcquireBuildSchedulingLockCallCount).Should(Equal(0))
		})

		It("does not start scheduling builds", func() {
			Consistently(scheduler.TryNextPendingBuildCallCount).Should(Equal(0))
			Consistently(scheduler.BuildLatestInputsCallCount).Should(Equal(0))
		})
	})
})
