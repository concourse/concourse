package scheduler_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/db/lock/lockfakes"
	. "github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/schedulerfakes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner", func() {
	var (
		fakePipeline *dbfakes.FakePipeline
		scheduler    *schedulerfakes.FakeBuildScheduler
		noop         bool

		lock *lockfakes.FakeLock

		initialConfig atc.Config

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
			BuildOutputs: []algorithm.BuildOutput{
				{
					ResourceVersion: algorithm.ResourceVersion{
						VersionID:  1,
						ResourceID: 2,
					},
					BuildID: 3,
					JobID:   4,
				},
				{
					ResourceVersion: algorithm.ResourceVersion{
						VersionID:  1,
						ResourceID: 2,
					},
					BuildID: 7,
					JobID:   8,
				},
			},
		}

		fakePipeline.LoadVersionsDBReturns(someVersions, nil)

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
		fakeJob1 = new(dbfakes.FakeJob)
		fakeJob1.NameReturns("some-job")
		fakeJob2 = new(dbfakes.FakeJob)
		fakeJob2.NameReturns("some-other-job")

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
		fakePipeline.AcquireSchedulingLockReturns(lock, true, nil)
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

	It("signs the scheduling lock for the pipeline", func() {
		Eventually(fakePipeline.AcquireSchedulingLockCallCount).Should(BeNumerically(">=", 1))

		_, duration := fakePipeline.AcquireSchedulingLockArgsForCall(0)
		Expect(duration).To(Equal(100 * time.Millisecond))
	})

	Context("when it can't get the lock", func() {
		BeforeEach(func() {
			fakePipeline.AcquireSchedulingLockReturns(nil, false, nil)
		})

		It("does not do any scheduling", func() {
			Eventually(fakePipeline.AcquireSchedulingLockCallCount).Should(Equal(2))

			Expect(scheduler.ScheduleCallCount()).To(BeZero())
		})
	})

	Context("when getting the lock blows up", func() {
		BeforeEach(func() {
			fakePipeline.AcquireSchedulingLockReturns(nil, false, errors.New(":3"))
		})

		It("does not do any scheduling", func() {
			Eventually(fakePipeline.AcquireSchedulingLockCallCount).Should(Equal(2))

			Expect(scheduler.ScheduleCallCount()).To(BeZero())
		})
	})

	It("schedules pending builds", func() {
		Eventually(scheduler.ScheduleCallCount).Should(Equal(2))

		_, versions, jobs, resources, resourceTypes := scheduler.ScheduleArgsForCall(0)
		Expect(versions).To(Equal(someVersions))
		Expect(jobs).To(Equal([]db.Job{fakeJob1, fakeJob2}))
		Expect(resources).To(Equal(db.Resources{fakeResource1, fakeResource2}))
		Expect(resourceTypes).To(Equal(versionedResourceTypes))
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
