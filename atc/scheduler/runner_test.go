package scheduler_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/algorithm"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	. "github.com/concourse/concourse/atc/scheduler"
	"github.com/concourse/concourse/atc/scheduler/schedulerfakes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner", func() {
	var (
		fakePipeline *dbfakes.FakePipeline
		scheduler    *schedulerfakes.FakeBuildScheduler
		noop         bool

		lock *lockfakes.FakeLock

		someVersions *algorithm.VersionsDB

		process ifrit.Process

		versionedResourceTypes atc.VersionedResourceTypes
		fakeJob1               *dbfakes.FakeJob
		fakeJob2               *dbfakes.FakeJob
		fakeResource1          *dbfakes.FakeResource
		fakeResource2          *dbfakes.FakeResource
	)

	BeforeEach(func() {
		fakePipeline = new(dbfakes.FakePipeline)
		fakePipeline.NameReturns("some-pipeline")

		versionedResourceTypes = atc.VersionedResourceTypes{
			atc.VersionedResourceType{
				ResourceType: atc.ResourceType{
					Name:   "some-resource-1",
					Type:   "some-base-type-1",
					Source: atc.Source{"some": "source-1"},
				},
				Version: atc.Version{"some": "version-1"},
			},
			atc.VersionedResourceType{
				ResourceType: atc.ResourceType{
					Name:   "some-resource-2",
					Type:   "some-base-type-2",
					Source: atc.Source{"some": "source-2"},
				},
				Version: atc.Version{"some": "version-2"},
			},
			atc.VersionedResourceType{
				ResourceType: atc.ResourceType{
					Name:   "some-resource-3",
					Type:   "some-base-type-3",
					Source: atc.Source{"some": "source-3"},
				},
				Version: atc.Version{"some": "version-3"},
			},
		}

		fakePipeline.ResourceTypesReturns([]db.ResourceType{
			fakeDBResourceType(versionedResourceTypes[0]),
			fakeDBResourceType(versionedResourceTypes[1]),
			fakeDBResourceType(versionedResourceTypes[2]),
		}, nil)

		scheduler = new(schedulerfakes.FakeBuildScheduler)
		noop = false

		someVersions = &algorithm.VersionsDB{
			ResourceIDs: map[string]int{"resource": 2},
			JobIDs:      map[string]int{"job-1": 4, "job-8": 8},
		}

		fakePipeline.LoadVersionsDBReturns(someVersions, nil)

		fakeJob1 = new(dbfakes.FakeJob)
		fakeJob1.NameReturns("some-job")
		fakeJob1.ReloadReturns(true, nil)
		fakeJob2 = new(dbfakes.FakeJob)
		fakeJob2.NameReturns("some-other-job")
		fakeJob2.ReloadReturns(true, nil)

		fakeResource1 = new(dbfakes.FakeResource)
		fakeResource1.NameReturns("some-resource")
		fakeResource1.TypeReturns("git")
		fakeResource1.SourceReturns(atc.Source{"uri": "git://some-resource"})
		fakeResource2 = new(dbfakes.FakeResource)
		fakeResource2.NameReturns("some-dependant-resource")
		fakeResource2.TypeReturns("git")
		fakeResource2.SourceReturns(atc.Source{"uri": "git://some-dependant-resource"})

		fakePipeline.JobsReturns([]db.Job{fakeJob1, fakeJob2}, nil)
		fakePipeline.ResourcesReturns(db.Resources{fakeResource1, fakeResource2}, nil)
		fakePipeline.ReloadReturns(true, nil)

		lock = new(lockfakes.FakeLock)
	})

	JustBeforeEach(func() {
		process = ginkgomon.Invoke(&Runner{
			Logger:    lagertest.NewTestLogger("test"),
			Pipeline:  fakePipeline,
			Scheduler: scheduler,
			Noop:      noop,
			Interval:  100 * time.Millisecond,
		})
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	It("loads up the versionsDB and grabs all the pipeline jobs", func() {
		Eventually(fakePipeline.LoadVersionsDBCallCount).Should(Equal(1))
		Eventually(fakePipeline.JobsCallCount).Should(Equal(1))
	})

	Context("when there are multiple jobs", func() {
		It("tries to acquire the scheduling lock for each job", func() {
			Eventually(fakeJob1.AcquireSchedulingLockCallCount).Should(BeNumerically(">=", 1))
			Eventually(fakeJob2.AcquireSchedulingLockCallCount).Should(BeNumerically(">=", 1))

			_, duration := fakeJob1.AcquireSchedulingLockArgsForCall(0)
			Expect(duration).To(Equal(100 * time.Millisecond))

			_, duration = fakeJob2.AcquireSchedulingLockArgsForCall(0)
			Expect(duration).To(Equal(100 * time.Millisecond))
		})

		Context("when it can't get the lock", func() {
			BeforeEach(func() {
				fakeJob1.AcquireSchedulingLockReturns(nil, false, nil)
			})

			It("does not do any scheduling", func() {
				Eventually(fakeJob1.AcquireSchedulingLockCallCount).Should(Equal(2))

				Expect(scheduler.ScheduleCallCount()).To(BeZero())
			})
		})

		Context("when getting the lock blows up", func() {
			BeforeEach(func() {
				fakeJob1.AcquireSchedulingLockReturns(nil, false, errors.New(":3"))
			})

			It("does not do any scheduling", func() {
				Eventually(fakeJob1.AcquireSchedulingLockCallCount).Should(Equal(2))

				Expect(scheduler.ScheduleCallCount()).To(BeZero())
			})
		})

		Context("when getting both locks succeeds", func() {
			BeforeEach(func() {
				fakeJob1.AcquireSchedulingLockReturns(lock, true, nil)
				fakeJob2.AcquireSchedulingLockReturns(lock, true, nil)
			})

			It("schedules pending builds", func() {
				Eventually(scheduler.ScheduleCallCount).Should(Equal(2))

				jobs := []string{}
				_, versions, job, resources, resourceTypes := scheduler.ScheduleArgsForCall(0)
				Expect(versions).To(Equal(someVersions))
				Expect(resources).To(Equal(db.Resources{fakeResource1, fakeResource2}))
				Expect(resourceTypes).To(Equal(versionedResourceTypes))
				jobs = append(jobs, job.Name())

				_, versions, job, resources, resourceTypes = scheduler.ScheduleArgsForCall(1)
				Expect(versions).To(Equal(someVersions))
				Expect(resources).To(Equal(db.Resources{fakeResource1, fakeResource2}))
				Expect(resourceTypes).To(Equal(versionedResourceTypes))
				jobs = append(jobs, job.Name())

				Expect(jobs).To(ConsistOf([]string{"some-job", "some-other-job"}))
			})
		})

		Context("when acquiring one job lock succeeds", func() {
			BeforeEach(func() {
				fakeJob1.AcquireSchedulingLockReturns(nil, false, nil)
				fakeJob2.AcquireSchedulingLockReturns(lock, true, nil)
			})

			It("schedules pending builds for one job", func() {
				Eventually(scheduler.ScheduleCallCount).Should(Equal(1))

				_, versions, job, resources, resourceTypes := scheduler.ScheduleArgsForCall(0)
				Expect(versions).To(Equal(someVersions))
				Expect(job).To(Equal(fakeJob2))
				Expect(resources).To(Equal(db.Resources{fakeResource1, fakeResource2}))
				Expect(resourceTypes).To(Equal(versionedResourceTypes))
			})
		})
	})

	Context("when in noop mode", func() {
		BeforeEach(func() {
			noop = true
		})

		It("does not start scheduling builds", func() {
			Consistently(scheduler.ScheduleCallCount).Should(Equal(0))
		})
	})

	eventualReloadConfigStubWith := func(found bool, err error) func() (bool, error) {
		calls := 0

		return func() (bool, error) {
			if calls == 1 {
				return found, err
			}

			calls++

			return true, nil
		}
	}

	Context("when the pipeline is destroyed", func() {
		BeforeEach(func() {
			fakePipeline.ReloadStub = eventualReloadConfigStubWith(false, nil)
		})

		It("exits", func() {
			Eventually(process.Wait()).Should(Receive())
		})
	})

	Context("when getting the config fails for some other reason", func() {
		BeforeEach(func() {
			fakePipeline.ReloadStub = eventualReloadConfigStubWith(false, errors.New("idk lol"))
		})

		It("keeps on truckin'", func() {
			Eventually(fakePipeline.ReloadCallCount).Should(BeNumerically(">=", 2))
		})
	})
})

func fakeDBResourceType(t atc.VersionedResourceType) *dbfakes.FakeResourceType {
	fake := new(dbfakes.FakeResourceType)
	fake.NameReturns(t.Name)
	fake.TypeReturns(t.Type)
	fake.SourceReturns(t.Source)
	fake.VersionReturns(t.Version)
	return fake
}
