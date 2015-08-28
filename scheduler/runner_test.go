package scheduler_test

import (
	"errors"
	"sync"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	dbfakes "github.com/concourse/atc/db/fakes"
	. "github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/fakes"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner", func() {
	var (
		locker     *fakes.FakeLocker
		pipelineDB *dbfakes.FakePipelineDB
		scheduler  *fakes.FakeBuildScheduler
		noop       bool

		lock *dbfakes.FakeLock

		initialConfig atc.Config

		someVersions algorithm.VersionsDB

		process ifrit.Process
	)

	BeforeEach(func() {
		locker = new(fakes.FakeLocker)
		pipelineDB = new(dbfakes.FakePipelineDB)
		pipelineDB.GetPipelineNameReturns("some-pipeline")
		scheduler = new(fakes.FakeBuildScheduler)
		noop = false

		someVersions = []algorithm.BuildOutput{
			{VersionID: 1, ResourceID: 2, BuildID: 3, JobID: 4},
			{VersionID: 5, ResourceID: 6, BuildID: 7, JobID: 8},
		}
		pipelineDB.LoadVersionsDBReturns(someVersions, nil)

		scheduler.TryNextPendingBuildStub = func(lager.Logger, algorithm.VersionsDB, atc.JobConfig, atc.ResourceConfigs) Waiter {
			return new(sync.WaitGroup)
		}

		initialConfig = atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "some-job",
				},
				{
					Name: "some-other-job",
				},
			},

			Resources: atc.ResourceConfigs{
				{
					Name:   "some-resource",
					Type:   "git",
					Source: atc.Source{"uri": "git://some-resource"},
				},
				{
					Name:   "some-dependant-resource",
					Type:   "git",
					Source: atc.Source{"uri": "git://some-dependant-resource"},
				},
			},
		}

		pipelineDB.GetConfigReturns(initialConfig, 1, nil)

		lock = new(dbfakes.FakeLock)
		locker.AcquireWriteLockImmediatelyReturns(lock, nil)
	})

	JustBeforeEach(func() {
		process = ginkgomon.Invoke(&Runner{
			Logger:    lagertest.NewTestLogger("test"),
			Locker:    locker,
			DB:        pipelineDB,
			Scheduler: scheduler,
			Noop:      noop,
			Interval:  100 * time.Millisecond,
		})
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	It("acquires the build scheduling lock for the pipeline", func() {
		Eventually(locker.AcquireWriteLockImmediatelyCallCount).Should(BeNumerically(">=", 1))

		lock := locker.AcquireWriteLockImmediatelyArgsForCall(0)
		Ω(lock).Should(Equal([]db.NamedLock{db.PipelineSchedulingLock("some-pipeline")}))
	})

	Context("when it can't get the lock", func() {
		BeforeEach(func() {
			locker.AcquireWriteLockImmediatelyStub = func(locks []db.NamedLock) (db.Lock, error) {
				return nil, errors.New("can't aqcuire lock")
			}
		})

		It("does not do any scheduling", func() {
			Eventually(locker.AcquireWriteLockImmediatelyCallCount).Should(Equal(2))

			Ω(scheduler.TryNextPendingBuildCallCount()).Should(BeZero())
			Ω(scheduler.BuildLatestInputsCallCount()).Should(BeZero())
		})
	})

	It("schedules pending builds", func() {
		Eventually(scheduler.TryNextPendingBuildCallCount).Should(Equal(2))

		_, versions, firstJob, resources := scheduler.TryNextPendingBuildArgsForCall(0)
		Ω(versions).Should(Equal(someVersions))
		Ω(resources).Should(Equal(initialConfig.Resources))

		_, versions, secondJob, resources := scheduler.TryNextPendingBuildArgsForCall(1)
		Ω(versions).Should(Equal(someVersions))
		Ω(resources).Should(Equal(initialConfig.Resources))

		Ω([]string{firstJob.Name, secondJob.Name}).Should(ConsistOf([]string{"some-job", "some-other-job"}))
	})

	Context("when pending builds are being tried", func() {
		var concurrent *sync.WaitGroup

		BeforeEach(func() {
			concurrent = new(sync.WaitGroup)
			concurrent.Add(2)

			scheduler.TryNextPendingBuildStub = func(lager.Logger, algorithm.VersionsDB, atc.JobConfig, atc.ResourceConfigs) Waiter {
				concurrent.Done()
				concurrent.Wait()
				return new(sync.WaitGroup)
			}
		})

		It("tries in parallel with others", func() {
			concurrent.Wait()
		})
	})

	It("schedules builds for new inputs using the given versions dataset", func() {
		Eventually(scheduler.BuildLatestInputsCallCount).Should(Equal(2))

		_, versions, firstJob, resources := scheduler.BuildLatestInputsArgsForCall(0)
		Ω(versions).Should(Equal(someVersions))
		Ω(resources).Should(Equal(initialConfig.Resources))

		_, versions, secondJob, resources := scheduler.BuildLatestInputsArgsForCall(1)
		Ω(versions).Should(Equal(someVersions))
		Ω(resources).Should(Equal(initialConfig.Resources))

		Ω([]string{firstJob.Name, secondJob.Name}).Should(ConsistOf([]string{"some-job", "some-other-job"}))
	})

	Context("when latest inputs are being built", func() {
		var concurrent *sync.WaitGroup

		BeforeEach(func() {
			concurrent = new(sync.WaitGroup)
			concurrent.Add(2)

			scheduler.BuildLatestInputsStub = func(lager.Logger, algorithm.VersionsDB, atc.JobConfig, atc.ResourceConfigs) error {
				concurrent.Done()
				concurrent.Wait()
				return nil
			}
		})

		It("tries in parallel with others", func() {
			concurrent.Wait()
		})
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

	failingGetConfigStubWith := func(err error) func() (atc.Config, db.ConfigVersion, error) {
		calls := 0

		return func() (atc.Config, db.ConfigVersion, error) {
			if calls == 1 {
				return atc.Config{}, 0, db.ErrPipelineNotFound
			}

			calls += 1

			return initialConfig, 1, nil
		}
	}

	Context("when the pipeline is destroyed", func() {
		BeforeEach(func() {
			pipelineDB.GetConfigStub = failingGetConfigStubWith(db.ErrPipelineNotFound)
		})

		It("exits", func() {
			Eventually(process.Wait()).Should(Receive())
		})
	})

	Context("when getting the config fails for some other reason", func() {
		BeforeEach(func() {
			pipelineDB.GetConfigStub = failingGetConfigStubWith(errors.New("idk lol"))
		})

		It("keeps on truckin'", func() {
			Eventually(pipelineDB.GetConfigCallCount).Should(BeNumerically(">=", 2))
		})
	})
})
