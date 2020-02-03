package scheduler_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	. "github.com/concourse/concourse/atc/scheduler"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
	"github.com/concourse/concourse/atc/scheduler/schedulerfakes"
	"github.com/hashicorp/go-multierror"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner", func() {
	var (
		fakePipeline  *dbfakes.FakePipeline
		fakeScheduler *schedulerfakes.FakeBuildScheduler

		lock *lockfakes.FakeLock

		fakeJobFactory *dbfakes.FakeJobFactory
		fakeJob1       *dbfakes.FakeJob
		fakeJob2       *dbfakes.FakeJob
		fakeJob3       *dbfakes.FakeJob
		fakeResource1  *dbfakes.FakeResource
		fakeResource2  *dbfakes.FakeResource

		expectedJobsMap algorithm.NameToIDMap

		job1RequestedTime time.Time
		job2RequestedTime time.Time
		job3RequestedTime time.Time

		schedulerRunner Runner
		schedulerErr    error
	)

	BeforeEach(func() {
		fakeScheduler = new(schedulerfakes.FakeBuildScheduler)
		fakeJobFactory = new(dbfakes.FakeJobFactory)

		expectedJobsMap = algorithm.NameToIDMap{
			"some-job":        1,
			"some-other-job":  2,
			"unscheduled-job": 3,
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
			fakeJobFactory,
			fakeScheduler,
			1,
		)
	})

	JustBeforeEach(func() {
		schedulerErr = schedulerRunner.Run(context.TODO())
	})

	It("loads up all the jobs to schedule", func() {
		Expect(fakeJobFactory.JobsToScheduleCallCount()).To(Equal(1))
	})

	Context("when there is one pipeline and two jobs that need to be scheduled", func() {
		BeforeEach(func() {
			fakePipeline = new(dbfakes.FakePipeline)
			fakePipeline.IDReturns(1)
			fakePipeline.NameReturns("fake-pipeline")
			fakePipeline.ReloadReturns(true, nil)

			job1RequestedTime = time.Now()
			job2RequestedTime = time.Now().Add(time.Minute)

			fakeJob1 = new(dbfakes.FakeJob)
			fakeJob1.IDReturns(1)
			fakeJob1.NameReturns("some-job")
			fakeJob1.ReloadReturns(true, nil)
			fakeJob1.PipelineIDReturns(1)
			fakeJob1.ScheduleRequestedTimeReturns(job1RequestedTime)
			fakeJob2 = new(dbfakes.FakeJob)
			fakeJob2.IDReturns(2)
			fakeJob2.NameReturns("some-other-job")
			fakeJob2.ReloadReturns(true, nil)
			fakeJob2.PipelineIDReturns(1)
			fakeJob2.ScheduleRequestedTimeReturns(job2RequestedTime)

			fakeJobFactory.JobsToScheduleReturns([]db.Job{fakeJob1, fakeJob2}, nil)
		})

		It("finds corresponding pipeline for job", func() {
			Eventually(fakeJob1.PipelineCallCount).Should(Equal(1))
		})

		Context("when finding pipeline for job succeeds", func() {
			BeforeEach(func() {
				fakeJob1.PipelineReturns(fakePipeline, true, nil)
				fakeJob2.PipelineReturns(fakePipeline, true, nil)
			})

			It("loads up the jobs", func() {
				Eventually(fakePipeline.JobsCallCount).Should(Equal(1))
			})

			Context("when the loading of jobs is successful", func() {
				BeforeEach(func() {
					fakeJob3 := new(dbfakes.FakeJob)
					fakeJob3.IDReturns(3)
					fakeJob3.NameReturns("unscheduled-job")

					fakePipeline.JobsReturns([]db.Job{fakeJob1, fakeJob2, fakeJob3}, nil)
				})

				It("loads up the resourcevs", func() {
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
								Expect(schedulerErr).ToNot(HaveOccurred())
								Eventually(fakeJob1.AcquireSchedulingLockCallCount).Should(Equal(1))
								Eventually(fakeJob1.UpdateLastScheduledCallCount).Should(Equal(0))
								Eventually(fakeScheduler.ScheduleCallCount).Should(BeZero())
							})
						})

						Context("when getting the lock blows up", func() {
							BeforeEach(func() {
								fakeJob1.AcquireSchedulingLockReturns(nil, false, errors.New(":3"))
							})

							It("does not do any scheduling", func() {
								Expect(schedulerErr).ToNot(HaveOccurred())
								Eventually(fakeJob1.AcquireSchedulingLockCallCount).Should(Equal(1))
								Eventually(fakeJob1.UpdateLastScheduledCallCount).Should(Equal(0))
								Eventually(fakeScheduler.ScheduleCallCount).Should(BeZero())
							})
						})

						Context("when getting both locks succeeds", func() {
							BeforeEach(func() {
								fakeJob1.AcquireSchedulingLockReturns(lock, true, nil)
								fakeJob2.AcquireSchedulingLockReturns(lock, true, nil)
							})

							It("reloads the job", func() {
								Eventually(fakeJob1.ReloadCallCount).Should(Equal(1))
								Eventually(fakeJob2.ReloadCallCount).Should(Equal(1))
							})

							Context("when reloading the job succeeds", func() {
								BeforeEach(func() {
									fakeJob1.ReloadReturns(true, nil)
									fakeJob2.ReloadReturns(true, nil)
								})

								It("schedules pending builds", func() {
									Eventually(fakeScheduler.ScheduleCallCount).Should(Equal(2))

									jobs := []string{}
									_, _, pipeline, job, resources, jobsMap := fakeScheduler.ScheduleArgsForCall(0)
									Expect(pipeline.Name()).To(Equal("fake-pipeline"))
									Expect(resources).To(Equal(db.Resources{fakeResource1, fakeResource2}))
									Expect(jobsMap).To(Equal(expectedJobsMap))
									jobs = append(jobs, job.Name())

									_, _, pipeline, job, resources, jobsMap = fakeScheduler.ScheduleArgsForCall(1)
									Expect(pipeline.Name()).To(Equal("fake-pipeline"))
									Expect(resources).To(Equal(db.Resources{fakeResource1, fakeResource2}))
									Expect(jobsMap).To(Equal(expectedJobsMap))
									jobs = append(jobs, job.Name())

									Expect(jobs).To(ConsistOf([]string{"some-job", "some-other-job"}))
								})

								Context("when all jobs scheduling succeeds", func() {
									BeforeEach(func() {
										fakeScheduler.ScheduleReturns(false, nil)
									})

									It("updates last schedule", func() {
										Expect(schedulerErr).ToNot(HaveOccurred())

										Eventually(fakeJob1.UpdateLastScheduledCallCount).Should(Equal(1))
										Eventually(fakeJob2.UpdateLastScheduledCallCount).Should(Equal(1))
										Expect(fakeJob1.UpdateLastScheduledArgsForCall(0)).To(Equal(job1RequestedTime))
										Expect(fakeJob2.UpdateLastScheduledArgsForCall(0)).To(Equal(job2RequestedTime))
									})
								})

								Context("when job scheduling fails", func() {
									BeforeEach(func() {
										fakeScheduler.ScheduleReturnsOnCall(0, false, errors.New("error"))
										fakeScheduler.ScheduleReturnsOnCall(1, false, nil)
									})

									It("does not update last scheduled", func() {
										Expect(schedulerErr).To(HaveOccurred())
										Expect(schedulerErr).To(Equal(&multierror.Error{
											Errors: []error{
												fmt.Errorf("schedule job: %w", errors.New("error")),
											},
										}))
										Eventually(fakeJob1.UpdateLastScheduledCallCount).Should(Equal(0))
										Eventually(fakeJob2.UpdateLastScheduledCallCount).Should(Equal(1))
									})
								})

								Context("when there is no error but needs retry", func() {
									BeforeEach(func() {
										fakeScheduler.ScheduleReturnsOnCall(0, true, nil)
										fakeScheduler.ScheduleReturnsOnCall(1, false, nil)
									})

									It("does not update last scheduled for the job that needs retry", func() {
										Expect(schedulerErr).ToNot(HaveOccurred())
										Eventually(fakeJob1.UpdateLastScheduledCallCount).Should(Equal(0))
										Eventually(fakeJob2.UpdateLastScheduledCallCount).Should(Equal(1))
									})
								})
							})

							Context("when reloading the job fails", func() {
								BeforeEach(func() {
									fakeJob1.ReloadReturns(false, errors.New("disappointment"))
								})

								It("does not update last schedule", func() {
									Expect(schedulerErr).To(HaveOccurred())
									Expect(schedulerErr).To(Equal(&multierror.Error{
										Errors: []error{
											fmt.Errorf("reload job: %w", errors.New("disappointment")),
										},
									}))
									Eventually(fakeJob1.UpdateLastScheduledCallCount).Should(Equal(0))
									Eventually(fakeJob2.UpdateLastScheduledCallCount).Should(Equal(1))
								})
							})

							Context("when the job to reload is not found", func() {
								BeforeEach(func() {
									fakeJob1.ReloadReturns(false, nil)
								})

								It("does not update last schedule", func() {
									Expect(schedulerErr).ToNot(HaveOccurred())
									Eventually(fakeJob1.UpdateLastScheduledCallCount).Should(Equal(0))
									Eventually(fakeJob2.UpdateLastScheduledCallCount).Should(Equal(1))
								})
							})
						})

						Context("when acquiring one job lock succeeds", func() {
							BeforeEach(func() {
								fakeJob1.AcquireSchedulingLockReturns(nil, false, nil)
								fakeJob2.AcquireSchedulingLockReturns(lock, true, nil)
							})

							It("schedules pending builds for one job", func() {
								Expect(schedulerErr).ToNot(HaveOccurred())
								Eventually(fakeScheduler.ScheduleCallCount).Should(Equal(1))

								_, _, pipeline, job, resources, jobIDs := fakeScheduler.ScheduleArgsForCall(0)
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
						Expect(schedulerErr).ToNot(HaveOccurred())
						Eventually(fakeJob1.UpdateLastScheduledCallCount).Should(Equal(0))
						Eventually(fakeJob2.UpdateLastScheduledCallCount).Should(Equal(0))
						Eventually(fakeScheduler.ScheduleCallCount).Should(Equal(0))
					})
				})
			})

			Context("when the loading of the jobs fails", func() {
				BeforeEach(func() {
					fakePipeline.JobsReturns(nil, errors.New("jobs error"))
				})

				It("does not update last scheduled", func() {
					Expect(schedulerErr).ToNot(HaveOccurred())
					Eventually(fakeJob1.UpdateLastScheduledCallCount).Should(Equal(0))
					Eventually(fakeJob2.UpdateLastScheduledCallCount).Should(Equal(0))
					Eventually(fakeScheduler.ScheduleCallCount).Should(Equal(0))
				})
			})
		})

		Context("when pipeline for job could not be found", func() {
			BeforeEach(func() {
				fakeJob1.PipelineReturns(nil, false, nil)
			})

			It("should not error or update last scheduled", func() {
				Expect(schedulerErr).ToNot(HaveOccurred())
				Eventually(fakeJob1.UpdateLastScheduledCallCount).Should(Equal(0))
				Eventually(fakeScheduler.ScheduleCallCount).Should(Equal(0))
			})
		})

		Context("when finding pipeline fails", func() {
			BeforeEach(func() {
				fakeJob1.PipelineReturns(nil, false, errors.New("failed"))
			})

			It("return an error and not update last schedule", func() {
				Expect(schedulerErr).To(HaveOccurred())
				Expect(schedulerErr).To(Equal(fmt.Errorf("find pipeline for job: %w", errors.New("failed"))))

				Eventually(fakeJob1.UpdateLastScheduledCallCount).Should(Equal(0))
				Eventually(fakeScheduler.ScheduleCallCount).Should(Equal(0))
			})
		})

		Context("when there are multiple jobs and pipelines", func() {
			var fakePipeline2 *dbfakes.FakePipeline

			BeforeEach(func() {
				fakePipeline = new(dbfakes.FakePipeline)
				fakePipeline.NameReturns("fake-pipeline")
				fakePipeline.IDReturns(1)
				fakePipeline2 = new(dbfakes.FakePipeline)
				fakePipeline2.NameReturns("another-fake-pipeline")
				fakePipeline2.IDReturns(2)

				job1RequestedTime = time.Now()
				job2RequestedTime = time.Now().Add(time.Minute)
				job3RequestedTime = time.Now().Add(2 * time.Minute)

				fakeJob1 = new(dbfakes.FakeJob)
				fakeJob1.IDReturns(1)
				fakeJob1.NameReturns("some-job")
				fakeJob1.ReloadReturns(true, nil)
				fakeJob1.PipelineIDReturns(1)
				fakeJob1.PipelineReturns(fakePipeline, true, nil)
				fakeJob1.ScheduleRequestedTimeReturns(job1RequestedTime)
				fakeJob2 = new(dbfakes.FakeJob)
				fakeJob2.IDReturns(2)
				fakeJob2.NameReturns("some-other-job")
				fakeJob2.ReloadReturns(true, nil)
				fakeJob2.PipelineIDReturns(2)
				fakeJob2.PipelineReturns(fakePipeline2, true, nil)
				fakeJob2.ScheduleRequestedTimeReturns(job2RequestedTime)
				fakeJob3 = new(dbfakes.FakeJob)
				fakeJob3.IDReturns(3)
				fakeJob3.NameReturns("another-other-job")
				fakeJob3.ReloadReturns(true, nil)
				fakeJob3.PipelineIDReturns(2)
				fakeJob3.PipelineReturns(fakePipeline2, true, nil)
				fakeJob3.ScheduleRequestedTimeReturns(job3RequestedTime)

				fakeJobFactory.JobsToScheduleReturns([]db.Job{fakeJob1, fakeJob2, fakeJob3}, nil)

				fakeScheduler.ScheduleReturns(false, nil)
			})

			Context("when both pipelines successfully schedule", func() {
				BeforeEach(func() {
					fakeJob4 := new(dbfakes.FakeJob)
					fakeJob4.IDReturns(1)
					fakeJob4.NameReturns("unscheduled-job")

					fakePipeline.JobsReturns([]db.Job{fakeJob1, fakeJob4}, nil)
					fakePipeline.ResourcesReturns(db.Resources{fakeResource1}, nil)
					fakeJob1.AcquireSchedulingLockReturns(lock, true, nil)

					fakePipeline2.JobsReturns([]db.Job{fakeJob2, fakeJob3}, nil)
					fakePipeline2.ResourcesReturns(db.Resources{fakeResource2}, nil)
					fakeJob2.AcquireSchedulingLockReturns(lock, true, nil)
					fakeJob3.AcquireSchedulingLockReturns(lock, true, nil)
				})

				It("all three jobs update the last scheduled", func() {
					Expect(schedulerErr).ToNot(HaveOccurred())
					Eventually(fakeScheduler.ScheduleCallCount).Should(Equal(3))

					Eventually(fakeJob1.UpdateLastScheduledCallCount).Should(Equal(1))
					Eventually(fakeJob2.UpdateLastScheduledCallCount).Should(Equal(1))
					Eventually(fakeJob3.UpdateLastScheduledCallCount).Should(Equal(1))

					Eventually(fakeJob1.UpdateLastScheduledArgsForCall(0)).Should(Equal(job1RequestedTime))
					Eventually(fakeJob2.UpdateLastScheduledArgsForCall(0)).Should(Equal(job2RequestedTime))
					Eventually(fakeJob3.UpdateLastScheduledArgsForCall(0)).Should(Equal(job3RequestedTime))
				})
			})

			Context("when the first pipeline fails to schedule", func() {
				BeforeEach(func() {
					fakePipeline.JobsReturns(nil, errors.New("error"))

					fakePipeline2.JobsReturns([]db.Job{fakeJob2, fakeJob3}, nil)
					fakePipeline2.ResourcesReturns(db.Resources{fakeResource2}, nil)
					fakeJob2.AcquireSchedulingLockReturns(lock, true, nil)
					fakeJob3.AcquireSchedulingLockReturns(lock, true, nil)
				})

				It("second pipeline still successfully schedules and updates last scheduled", func() {
					Expect(schedulerErr).NotTo(HaveOccurred())
					Eventually(fakeScheduler.ScheduleCallCount).Should(Equal(2))
					Eventually(fakeJob1.UpdateLastScheduledCallCount).Should(Equal(0))
					Eventually(fakeJob2.UpdateLastScheduledCallCount).Should(Equal(1))
					Eventually(fakeJob3.UpdateLastScheduledCallCount).Should(Equal(1))
				})
			})

			Context("when the two jobs fail to schedule", func() {
				BeforeEach(func() {
					fakePipeline.JobsReturns([]db.Job{fakeJob1}, nil)
					fakePipeline.ResourcesReturns(db.Resources{fakeResource1}, nil)
					fakeJob1.AcquireSchedulingLockReturns(lock, true, nil)
					fakeJob1.ReloadReturns(false, errors.New("error-1"))

					fakePipeline2.JobsReturns([]db.Job{fakeJob2, fakeJob3}, nil)
					fakePipeline2.ResourcesReturns(db.Resources{fakeResource2}, nil)
					fakeJob2.AcquireSchedulingLockReturns(lock, true, nil)
					fakeJob3.AcquireSchedulingLockReturns(lock, true, nil)
					fakeJob3.ReloadReturns(false, errors.New("error-3"))
				})

				It("schedules the remaining job", func() {
					Expect(schedulerErr).To(HaveOccurred())
					Expect(schedulerErr.Error()).To(ContainSubstring("error-1"))
					Expect(schedulerErr.Error()).To(ContainSubstring("error-3"))
					Expect(schedulerErr.Error()).ToNot(ContainSubstring("error-2"))
					Eventually(fakeScheduler.ScheduleCallCount).Should(Equal(1))
					Eventually(fakeJob1.UpdateLastScheduledCallCount).Should(Equal(0))
					Eventually(fakeJob2.UpdateLastScheduledCallCount).Should(Equal(1))
					Eventually(fakeJob3.UpdateLastScheduledCallCount).Should(Equal(0))
				})
			})
		})
	})

	Context("when finding jobs to schedule fails", func() {
		BeforeEach(func() {
			fakeJobFactory.JobsToScheduleReturns(nil, errors.New("disaster"))
		})

		It("returns an error", func() {
			Expect(schedulerErr).To(Equal(fmt.Errorf("find jobs to schedule: %w", errors.New("disaster"))))
		})
	})
})
