package scheduler_test

import (
	"errors"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	dbfakes "github.com/concourse/atc/db/dbfakes"
	. "github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/schedulerfakes"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner", func() {
	var (
		pipelineDB *dbfakes.FakePipelineDB
		scheduler  *schedulerfakes.FakeBuildScheduler
		noop       bool

		lease *dbfakes.FakeLease

		initialConfig atc.Config

		someVersions *algorithm.VersionsDB

		process ifrit.Process
	)

	BeforeEach(func() {
		pipelineDB = new(dbfakes.FakePipelineDB)
		pipelineDB.GetPipelineNameReturns("some-pipeline")
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

		pipelineDB.LoadVersionsDBReturns(someVersions, nil)

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

		pipelineDB.GetConfigReturns(initialConfig, 1, true, nil)

		lease = new(dbfakes.FakeLease)
		pipelineDB.LeaseSchedulingReturns(lease, true, nil)
	})

	JustBeforeEach(func() {
		process = ginkgomon.Invoke(&Runner{
			Logger:    lagertest.NewTestLogger("test"),
			DB:        pipelineDB,
			Scheduler: scheduler,
			Noop:      noop,
			Interval:  100 * time.Millisecond,
		})
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	It("signs the scheduling lease for the pipeline", func() {
		Eventually(pipelineDB.LeaseSchedulingCallCount).Should(BeNumerically(">=", 1))

		_, duration := pipelineDB.LeaseSchedulingArgsForCall(0)
		Expect(duration).To(Equal(100 * time.Millisecond))
	})

	Context("when it can't get the lease", func() {
		BeforeEach(func() {
			pipelineDB.LeaseSchedulingReturns(nil, false, nil)
		})

		It("does not do any scheduling", func() {
			Eventually(pipelineDB.LeaseSchedulingCallCount).Should(Equal(2))

			Expect(scheduler.ScheduleCallCount()).To(BeZero())
		})
	})

	Context("when getting the lease blows up", func() {
		BeforeEach(func() {
			pipelineDB.LeaseSchedulingReturns(nil, false, errors.New(":3"))
		})

		It("does not do any scheduling", func() {
			Eventually(pipelineDB.LeaseSchedulingCallCount).Should(Equal(2))

			Expect(scheduler.ScheduleCallCount()).To(BeZero())
		})
	})

	It("schedules pending builds", func() {
		Eventually(scheduler.ScheduleCallCount).Should(Equal(2))

		_, versions, job, resources, resourceTypes := scheduler.ScheduleArgsForCall(0)
		Expect(versions).To(Equal(someVersions))
		Expect(job).To(Equal(atc.JobConfig{Name: "some-job"}))
		Expect(resources).To(Equal(initialConfig.Resources))
		Expect(resourceTypes).To(Equal(initialConfig.ResourceTypes))

		_, versions, job, resources, resourceTypes = scheduler.ScheduleArgsForCall(1)
		Expect(versions).To(Equal(someVersions))
		Expect(job).To(Equal(atc.JobConfig{Name: "some-other-job"}))
		Expect(resources).To(Equal(initialConfig.Resources))
		Expect(resourceTypes).To(Equal(initialConfig.ResourceTypes))
	})

	Context("when in noop mode", func() {
		BeforeEach(func() {
			noop = true
		})

		It("does not start scheduling builds", func() {
			Consistently(scheduler.ScheduleCallCount).Should(Equal(0))
		})
	})

	failingGetConfigStubWith := func(found bool, err error) func() (atc.Config, db.ConfigVersion, bool, error) {
		calls := 0

		return func() (atc.Config, db.ConfigVersion, bool, error) {
			if calls == 1 {
				return atc.Config{}, 0, found, err
			}

			calls++

			return initialConfig, 1, true, nil
		}
	}

	Context("when the pipeline is destroyed", func() {
		BeforeEach(func() {
			pipelineDB.GetConfigStub = failingGetConfigStubWith(false, nil)
		})

		It("exits", func() {
			Eventually(process.Wait()).Should(Receive())
		})
	})

	Context("when getting the config fails for some other reason", func() {
		BeforeEach(func() {
			pipelineDB.GetConfigStub = failingGetConfigStubWith(false, errors.New("idk lol"))
		})

		It("keeps on truckin'", func() {
			Eventually(pipelineDB.GetConfigCallCount).Should(BeNumerically(">=", 2))
		})
	})
})
