package scheduler_test

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	. "github.com/concourse/concourse/atc/scheduler"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
	"github.com/concourse/concourse/atc/scheduler/schedulerfakes"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner", func() {
	var (
		fakePipelineFactory *dbfakes.FakePipelineFactory
		fakePipeline        *dbfakes.FakePipeline
		fakeScheduler       *schedulerfakes.FakeBuildScheduler

		lock *lockfakes.FakeLock

		fakeJob1      *dbfakes.FakeJob
		fakeJob2      *dbfakes.FakeJob
		fakeResource1 *dbfakes.FakeResource
		fakeResource2 *dbfakes.FakeResource

		expectedJobsMap algorithm.NameToIDMap

		pipelineRequestedTime time.Time

		schedulerRunner Runner
		schedulerErr    error
	)

	BeforeEach(func() {
		fakePipelineFactory = new(dbfakes.FakePipelineFactory)
		fakePipeline = new(dbfakes.FakePipeline)
		fakePipeline.NameReturns("fake-pipeline")
		fakePipeline.ReloadReturns(true, nil)

		pipelineRequestedTime = time.Now()
		fakePipeline.RequestedTimeReturns(pipelineRequestedTime)

		fakeScheduler = new(schedulerfakes.FakeBuildScheduler)

		fakeJob1 = new(dbfakes.FakeJob)
		fakeJob1.IDReturns(1)
		fakeJob1.NameReturns("some-job")
		fakeJob1.ReloadReturns(true, nil)
		fakeJob2 = new(dbfakes.FakeJob)
		fakeJob2.IDReturns(2)
		fakeJob2.NameReturns("some-other-job")
		fakeJob2.ReloadReturns(true, nil)

		expectedJobsMap = algorithm.NameToIDMap{
			"some-job":       1,
			"some-other-job": 2,
		}

		fakeResource1 = new(dbfakes.FakeResource)
		fakeResource1.NameReturns("some-resource")
		fakeResource1.TypeReturns("git")
		fakeResource1.SourceReturns(atc.Source{"uri": "git://some-resource"})
		fakeResource2 = new(dbfakes.FakeResource)
		fakeResource2.NameReturns("some-dependant-resource")
		fakeResource2.TypeReturns("git")
		fakeResource2.SourceReturns(atc.Source{"uri": "git://some-dependant-resource"})

		lock = new(lockfakes.FakeLock)

		schedulerRunner = NewRunner(
			lagertest.NewTestLogger("test"),
			fakePipelineFactory,
			fakeScheduler,
			32,
		)
	})

	JustBeforeEach(func() {
		schedulerErr = schedulerRunner.Run(context.TODO())
		Expect(schedulerErr).ToNot(HaveOccurred())
	})

	It("loads up all the pipelines to schedule", func() {
		Expect(fakePipelineFactory.PipelinesToScheduleCallCount()).To(Equal(1))
	})

	Context("when there is one pipeline", func() {
		BeforeEach(func() {
			fakePipelineFactory.PipelinesToScheduleReturns([]db.Pipeline{fakePipeline}, nil)
		})

		It("loads up the jobs for the pipeline", func() {
			Eventually(fakePipeline.JobsCallCount).Should(Equal(1))
		})

		Context("when the loading of jobs is successful", func() {
			BeforeEach(func() {
				fakePipeline.JobsReturns([]db.Job{fakeJob1, fakeJob2}, nil)
			})
			It("loads up the resources", func() {
				Eventually(fakePipeline.ResourcesCallCount).Should(Equal(1))
			})

			Context("when the loading of the resources is successful", func() {
				BeforeEach(func() {
					fakePipeline.ResourcesReturns(db.Resources{fakeResource1, fakeResource2}, nil)
				})
				Context("when there are multiple jobs", func() {
					It("tries to acquire the scheduling lock for each job", func() {
						Eventually(fakeJob1.AcquireSchedulingLockCallCount).Should(Equal(1))
						Eventually(fakeJob2.AcquireSchedulingLockCallCount).Should(Equal(1))
					})

					Context("when it can't get the lock", func() {
						BeforeEach(func() {
							fakeJob1.AcquireSchedulingLockReturns(nil, false, nil)
						})

						It("does not do any scheduling", func() {
							Eventually(fakeJob1.AcquireSchedulingLockCallCount).Should(Equal(1))

							Eventually(fakeScheduler.ScheduleCallCount).Should(BeZero())
						})
					})

					Context("when getting the lock blows up", func() {
						BeforeEach(func() {
							fakeJob1.AcquireSchedulingLockReturns(nil, false, errors.New(":3"))
						})

						It("does not do any scheduling", func() {
							Eventually(fakeJob1.AcquireSchedulingLockCallCount).Should(Equal(1))

							Eventually(fakeScheduler.ScheduleCallCount).Should(BeZero())
						})
					})

					Context("when getting both locks succeeds", func() {
						BeforeEach(func() {
							fakeJob1.AcquireSchedulingLockReturns(lock, true, nil)
							fakeJob2.AcquireSchedulingLockReturns(lock, true, nil)
						})

						It("schedules pending builds", func() {
							Eventually(fakeScheduler.ScheduleCallCount).Should(Equal(2))

							jobs := []string{}
							_, pipeline, job, resources, jobsMap := fakeScheduler.ScheduleArgsForCall(0)
							Expect(pipeline.Name()).To(Equal("fake-pipeline"))
							Expect(resources).To(Equal(db.Resources{fakeResource1, fakeResource2}))
							Expect(jobsMap).To(Equal(expectedJobsMap))
							jobs = append(jobs, job.Name())

							_, pipeline, job, resources, jobsMap = fakeScheduler.ScheduleArgsForCall(1)
							Expect(pipeline.Name()).To(Equal("fake-pipeline"))
							Expect(resources).To(Equal(db.Resources{fakeResource1, fakeResource2}))
							Expect(jobsMap).To(Equal(expectedJobsMap))
							jobs = append(jobs, job.Name())

							Expect(jobs).To(ConsistOf([]string{"some-job", "some-other-job"}))
						})

						Context("when job scheduling fails", func() {
							BeforeEach(func() {
								fakeScheduler.ScheduleReturnsOnCall(0, errors.New("error"))
							})

							It("only requests schedule once for the failed job", func() {
								Eventually(fakePipeline.RequestScheduleCallCount).Should(Equal(1))
							})
						})

						Context("when all jobs scheduling succeeds", func() {
							BeforeEach(func() {
								fakeScheduler.ScheduleReturns(nil)
							})

							It("does not request schedule", func() {
								Eventually(fakePipeline.RequestScheduleCallCount).Should(Equal(0))
							})
						})
					})

					Context("when acquiring one job lock succeeds", func() {
						BeforeEach(func() {
							fakeJob1.AcquireSchedulingLockReturns(nil, false, nil)
							fakeJob2.AcquireSchedulingLockReturns(lock, true, nil)
						})

						It("schedules pending builds for one job", func() {
							Eventually(fakeScheduler.ScheduleCallCount).Should(Equal(1))

							_, pipeline, job, resources, jobIDs := fakeScheduler.ScheduleArgsForCall(0)
							Expect(job).To(Equal(fakeJob2))
							Expect(resources).To(Equal(db.Resources{fakeResource1, fakeResource2}))
							Expect(pipeline).To(Equal(fakePipeline))
							Expect(jobIDs).To(Equal(expectedJobsMap))
						})
					})
				})
			})

			Context("when the loading of the resources fails", func() {
				BeforeEach(func() {
					fakePipeline.ResourcesReturns(nil, errors.New("resources error"))
				})

				It("does not update last scheduled", func() {
					Eventually(fakePipeline.UpdateLastScheduledCallCount).Should(Equal(0))
					Eventually(fakeScheduler.ScheduleCallCount).Should(Equal(0))
				})
			})
		})

		Context("when the loading of the jobs fails", func() {
			BeforeEach(func() {
				fakePipeline.JobsReturns(nil, errors.New("jobs error"))
			})

			It("does not update last scheduled", func() {
				Eventually(fakePipeline.UpdateLastScheduledCallCount).Should(Equal(0))
				Eventually(fakeScheduler.ScheduleCallCount).Should(Equal(0))
			})
		})

		Context("when there are multiple pipelines", func() {
			var fakePipeline2 *dbfakes.FakePipeline
			var pipeline2RequestedTime time.Time

			BeforeEach(func() {
				pipeline2RequestedTime = time.Now()

				fakePipeline2 = new(dbfakes.FakePipeline)
				fakePipeline2.NameReturns("another-fake-pipeline")
				fakePipeline2.ReloadReturns(true, nil)
				fakePipeline2.RequestedTimeReturns(pipeline2RequestedTime)

				fakePipelineFactory.PipelinesToScheduleReturns([]db.Pipeline{fakePipeline, fakePipeline2}, nil)
				fakeScheduler.ScheduleReturns(nil)
			})

			Context("when both pipelines successfully schedule", func() {
				BeforeEach(func() {
					fakePipeline.JobsReturns([]db.Job{fakeJob1}, nil)
					fakePipeline.ResourcesReturns(db.Resources{fakeResource1}, nil)
					fakeJob1.AcquireSchedulingLockReturns(lock, true, nil)

					fakePipeline2.JobsReturns([]db.Job{fakeJob2}, nil)
					fakePipeline2.ResourcesReturns(db.Resources{fakeResource2}, nil)
					fakeJob2.AcquireSchedulingLockReturns(lock, true, nil)
				})

				It("both update the last scheduled", func() {
					Eventually(fakeScheduler.ScheduleCallCount).Should(Equal(2))
					Eventually(fakePipeline.UpdateLastScheduledCallCount).Should(Equal(1))
					Eventually(fakePipeline2.UpdateLastScheduledCallCount).Should(Equal(1))

					Eventually(fakePipeline.UpdateLastScheduledArgsForCall(0)).Should(Equal(pipelineRequestedTime))
					Eventually(fakePipeline2.UpdateLastScheduledArgsForCall(0)).Should(Equal(pipeline2RequestedTime))
				})
			})

			Context("when the first pipeline fails to schedule", func() {
				BeforeEach(func() {
					fakePipeline.JobsReturns(nil, errors.New("error"))

					fakePipeline2.JobsReturns([]db.Job{fakeJob2}, nil)
					fakePipeline2.ResourcesReturns(db.Resources{fakeResource2}, nil)
					fakeJob2.AcquireSchedulingLockReturns(lock, true, nil)
				})

				It("second pipeline still successfully schedules and updates last scheduled", func() {
					Eventually(fakeScheduler.ScheduleCallCount).Should(Equal(1))
					Eventually(fakePipeline.UpdateLastScheduledCallCount).Should(Equal(0))
					Eventually(fakePipeline2.UpdateLastScheduledCallCount).Should(Equal(1))
					Eventually(fakePipeline2.UpdateLastScheduledArgsForCall(0)).Should(Equal(pipeline2RequestedTime))
				})
			})
		})
	})
})
