package scheduler_test

import (
	"errors"
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
		locker.AcquireWriteLockImmediatelyReturns(lock, nil)
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

	It("acquires the build scheduling lock for each job", func() {
		Eventually(locker.AcquireWriteLockImmediatelyCallCount).Should(Equal(2))

		job := locker.AcquireWriteLockImmediatelyArgsForCall(0)
		Ω(job).Should(Equal([]db.NamedLock{db.JobSchedulingLock("some-job")}))

		job = locker.AcquireWriteLockImmediatelyArgsForCall(1)
		Ω(job).Should(Equal([]db.NamedLock{db.JobSchedulingLock("some-other-job")}))
	})

	Context("whe it can't get the lock for the first job", func() {
		BeforeEach(func() {
			locker.AcquireWriteLockImmediatelyStub = func(locks []db.NamedLock) (db.Lock, error) {
				if locker.AcquireWriteLockImmediatelyCallCount() == 1 {
					return nil, errors.New("can't aqcuire lock")
				}
				return lock, nil
			}
		})

		It("follows on to the next job", func() {
			Eventually(locker.AcquireWriteLockImmediatelyCallCount).Should(Equal(2))

			job := scheduler.TryNextPendingBuildArgsForCall(0)
			Ω(job).Should(Equal(config.Job{Name: "some-other-job"}))
		})
	})

	It("tracks in-flight builds", func() {
		Eventually(scheduler.TrackInFlightBuildsCallCount).Should(Equal(1))
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

	Context("when in noop mode", func() {
		BeforeEach(func() {
			noop = true
		})

		It("does not start scheduling builds", func() {
			Consistently(scheduler.TryNextPendingBuildCallCount).Should(Equal(0))
			Consistently(scheduler.BuildLatestInputsCallCount).Should(Equal(0))
		})
	})
})
