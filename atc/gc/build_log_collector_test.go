package gc_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/gc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildLogCollector", func() {
	var (
		buildLogCollector     GcCollector
		fakePipelineFactory   *dbfakes.FakePipelineFactory
		fakePipelineLifecycle *dbfakes.FakePipelineLifecycle
		batchSize             int
		buildLogRetainCalc    BuildLogRetentionCalculator
	)

	BeforeEach(func() {
		fakePipelineFactory = new(dbfakes.FakePipelineFactory)
		fakePipelineLifecycle = new(dbfakes.FakePipelineLifecycle)
		batchSize = 5
		buildLogRetainCalc = NewBuildLogRetentionCalculator(0, 0, 0, 0)
	})

	JustBeforeEach(func() {
		buildLogCollector = NewBuildLogCollector(
			fakePipelineFactory,
			fakePipelineLifecycle,
			batchSize,
			buildLogRetainCalc,
			false,
		)
	})

	It("removes build events from deleted pipelines", func() {
		err := buildLogCollector.Run(context.TODO())
		Expect(err).ToNot(HaveOccurred())
		Expect(fakePipelineLifecycle.RemoveBuildEventsForDeletedPipelinesCallCount()).To(Equal(1))
	})

	Context("when removing build events from deleted pipelines fails", func() {
		BeforeEach(func() {
			fakePipelineLifecycle.RemoveBuildEventsForDeletedPipelinesReturns(errors.New("error"))
		})

		It("errors", func() {
			err := buildLogCollector.Run(context.TODO())
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when there is a pipeline", func() {
		var fakePipeline *dbfakes.FakePipeline

		BeforeEach(func() {
			fakePipeline = new(dbfakes.FakePipeline)
			fakePipeline.IDReturns(42)

			fakePipelineFactory.AllPipelinesReturns([]db.Pipeline{fakePipeline}, nil)
		})

		Context("when getting the dashboard fails", func() {
			var disaster error

			BeforeEach(func() {
				disaster = errors.New("sorry pal")
				fakePipeline.JobsReturns(nil, disaster)
			})

			It("returns the error", func() {
				err := buildLogCollector.Run(context.TODO())
				Expect(err).To(Equal(disaster))
			})
		})

		Context("when the dashboard has a job", func() {
			var fakeJob *dbfakes.FakeJob

			BeforeEach(func() {
				fakeJob = new(dbfakes.FakeJob)
				fakeJob.NameReturns("job-1")
				fakeJob.FirstLoggedBuildIDReturns(5)
				fakeJob.ConfigReturns(atc.JobConfig{
					BuildLogsToRetain: 2,
				}, nil)

				fakePipeline.JobsReturns([]db.Job{fakeJob}, nil)
			})

			Context("drain handling", func() {
				JustBeforeEach(func() {
					buildLogCollector = NewBuildLogCollector(
						fakePipelineFactory,
						fakePipelineLifecycle,
						batchSize,
						buildLogRetainCalc,
						true,
					)
				})
				BeforeEach(func() {
					page1 := db.Page{From: 5, Limit: 5}
					page2 := db.Page{From: 10, Limit: 5}
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == page1 {
							return []db.Build{sbDrained(9, false), sbDrained(8, false), sbDrained(7, true), sbDrained(6, false), sbDrained(5, true)}, db.Pagination{Newer: &page2}, nil
						} else if page == (db.Page{From: 10, Limit: 5}) {
							return []db.Build{sbDrained(11, true), sbDrained(10, true)}, db.Pagination{}, nil
						}
						Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						return []db.Build{}, db.Pagination{}, nil
					}

					fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)
					fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
				})

				JustBeforeEach(func() {
					err := buildLogCollector.Run(context.TODO())
					Expect(err).NotTo(HaveOccurred())
				})

				It("should not reap builds which have not been drained", func() {
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))

					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).Should(Not(ContainElement(6)))
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).Should(Not(ContainElement(8)))
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).Should(Not(ContainElement(9)))
				})

				It("should reap builds which have been drained", func() {
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))

					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).To(ConsistOf(7, 5))
				})

				It("should update first logged build id to the earliest non-drained build", func() {
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))

					Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(Equal(1))
					actualNewFirstLoggedBuildID := fakeJob.UpdateFirstLoggedBuildIDArgsForCall(0)
					Expect(actualNewFirstLoggedBuildID).To(Equal(6))
				})
			})

			Context("when drain has not been configured", func() {
				BeforeEach(func() {
					buildLogCollector = NewBuildLogCollector(
						fakePipelineFactory,
						fakePipelineLifecycle,
						batchSize,
						buildLogRetainCalc,
						false,
					)
					page1 := db.Page{From: 5, Limit: 5}
					page2 := db.Page{From: 10, Limit: 5}
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == page1 {
							return []db.Build{sbDrained(9, true), sbDrained(8, false), sbDrained(7, false), sbDrained(6, true), sbDrained(5, false)}, db.Pagination{Newer: &page2}, nil
						} else if page == page2 {
							return []db.Build{sbDrained(10, true)}, db.Pagination{}, nil
						}
						Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						return []db.Build{}, db.Pagination{}, nil
					}

					fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)
					fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
				})
				It("should reap builds if draining is not configured", func() {
					err := buildLogCollector.Run(context.TODO())
					Expect(err).NotTo(HaveOccurred())
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))

					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).To(ConsistOf(5, 6, 7, 8))

					Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(Equal(1))
					actualNewFirstLoggedBuildID := fakeJob.UpdateFirstLoggedBuildIDArgsForCall(0)
					Expect(actualNewFirstLoggedBuildID).To(Equal(9))
				})
			})

			Context("when deleting build events fails", func() {
				var disaster error

				BeforeEach(func() {
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == (db.Page{From: 5, Limit: 5}) {
							return []db.Build{sbDrained(8, false), sbDrained(7, true), sbDrained(6, false), sbDrained(5, false)}, db.Pagination{}, nil
						}
						Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						return []db.Build{}, db.Pagination{}, nil
					}

					disaster = errors.New("major malfunction")

					fakePipeline.DeleteBuildEventsByBuildIDsReturns(disaster)
				})

				It("returns the error", func() {
					err := buildLogCollector.Run(context.TODO())
					Expect(err).To(Equal(disaster))
				})

				It("does not update first logged build id", func() {
					buildLogCollector.Run(context.TODO())

					Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(BeZero())
				})
			})

			Context("when updating first logged build id fails", func() {
				var disaster error

				BeforeEach(func() {
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == (db.Page{From: 5, Limit: 5}) {
							return []db.Build{sbDrained(8, false), sbDrained(7, true), sbDrained(6, false), sbDrained(5, false)}, db.Pagination{}, nil
						}
						Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						return []db.Build{}, db.Pagination{}, nil
					}

					disaster = errors.New("major malfunction")

					fakeJob.UpdateFirstLoggedBuildIDReturns(disaster)
				})

				It("returns the error", func() {
					err := buildLogCollector.Run(context.TODO())
					Expect(err).To(Equal(disaster))
				})
			})

			Context("when the builds we want to reap are still running", func() {
				BeforeEach(func() {
					fakeJob.ConfigReturns(atc.JobConfig{
						BuildLogsToRetain: 3,
					}, nil)
					page1 := db.Page{From: 5, Limit: 5}
					page2 := db.Page{From: 10, Limit: 5}
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == page1 {
							return []db.Build{
								runningBuild(9),
								runningBuild(8),
								sb(7),
								sb(6),
								sb(5),
							}, db.Pagination{Newer: &page2}, nil
						} else if page == page2 {
							return []db.Build{sb(10)}, db.Pagination{}, nil
						} else {
							Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						}
						return nil, db.Pagination{}, nil
					}

					fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)

					fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
				})

				JustBeforeEach(func() {
					err := buildLogCollector.Run(context.TODO())
					Expect(err).NotTo(HaveOccurred())
				})

				It("reaps only not-running builds", func() {
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))
					actualBuildIDs := fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)
					Expect(actualBuildIDs).To(ConsistOf(5))
				})

				It("updates FirstLoggedBuildID to earliest non-reaped build", func() {
					Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(Equal(1))
					actualNewFirstLoggedBuildID := fakeJob.UpdateFirstLoggedBuildIDArgsForCall(0)
					Expect(actualNewFirstLoggedBuildID).To(Equal(6))
				})
			})

			Context("when no builds need to be reaped", func() {
				BeforeEach(func() {
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == (db.Page{From: 5, Limit: 5}) {
							return []db.Build{runningBuild(5)}, db.Pagination{}, nil
						} else {
							Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						}
						return nil, db.Pagination{}, nil
					}

					fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)

					fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
				})

				JustBeforeEach(func() {
					err := buildLogCollector.Run(context.TODO())
					Expect(err).NotTo(HaveOccurred())
				})

				It("doesn't reap any builds", func() {
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(BeZero())
				})

				It("doesn't update FirstLoggedBuildID", func() {
					Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(BeZero())
				})
			})

			Context("when no builds exist", func() {
				BeforeEach(func() {
					fakeJob.BuildsReturns(nil, db.Pagination{}, nil)

					fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)

					fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
				})

				It("doesn't reap any builds", func() {
					err := buildLogCollector.Run(context.TODO())
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(BeZero())
					Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(BeZero())
				})
			})

			Context("when getting the job builds fails", func() {
				var disaster error

				BeforeEach(func() {
					disaster = errors.New("major malfunction")

					fakeJob.BuildsReturns(nil, db.Pagination{}, disaster)
				})

				It("returns the error", func() {
					err := buildLogCollector.Run(context.TODO())
					Expect(err).To(Equal(disaster))
				})
			})

			Context("when only count is set", func() {
				BeforeEach(func() {
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == (db.Page{From: 5, Limit: 5}) {
							return []db.Build{sbTime(6, time.Now().Add(-23*time.Hour)), sbTime(5, time.Now().Add(-49*time.Hour))}, db.Pagination{}, nil
						}
						Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						return nil, db.Pagination{}, nil
					}

					fakeJob.ConfigReturns(atc.JobConfig{
						BuildLogRetention: &atc.BuildLogRetention{
							Builds: 1,
							Days:   0,
						},
					}, nil)

					fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)
					fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
				})

				It("should delete 1 build event", func() {
					err := buildLogCollector.Run(context.TODO())
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))
					actualBuildIDs := fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)
					Expect(actualBuildIDs).To(ConsistOf(5))
				})
			})

			Context("when only date is set", func() {
				BeforeEach(func() {
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == (db.Page{From: 5, Limit: 5}) {
							return []db.Build{sbTime(6, time.Now().Add(-23*time.Hour)), sbTime(5, time.Now().Add(-49*time.Hour))}, db.Pagination{}, nil
						}
						Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						return nil, db.Pagination{}, nil
					}

					fakeJob.ConfigReturns(atc.JobConfig{
						BuildLogRetention: &atc.BuildLogRetention{
							Builds: 0,
							Days:   3,
						},
					}, nil)

					fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)
					fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
				})

				It("should delete nothing, because of the date retention", func() {
					err := buildLogCollector.Run(context.TODO())
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(0))
				})
			})

			Context("when count and date are set > 0", func() {
				BeforeEach(func() {
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == (db.Page{From: 5, Limit: 5}) {
							return []db.Build{sbTime(6, time.Now().Add(-23*time.Hour)), sbTime(5, time.Now().Add(-49*time.Hour))}, db.Pagination{}, nil
						}
						Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						return nil, db.Pagination{}, nil
					}

					fakeJob.ConfigReturns(atc.JobConfig{
						BuildLogRetention: &atc.BuildLogRetention{
							Builds: 1,
							Days:   3,
						},
					}, nil)

					fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)
					fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
				})

				It("should delete 1 build, because of the builds retention", func() {
					err := buildLogCollector.Run(context.TODO())
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))
					actualBuildIDs := fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)
					Expect(actualBuildIDs).To(ConsistOf(5))
				})
			})

			Context("when only date is set", func() {
				BeforeEach(func() {
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == (db.Page{From: 5, Limit: 5}) {
							return []db.Build{sbTime(6, time.Now().Add(-23*time.Hour)), sbTime(5, time.Now().Add(-49*time.Hour))}, db.Pagination{}, nil
						}
						Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						return nil, db.Pagination{}, nil
					}

					fakeJob.ConfigReturns(atc.JobConfig{
						BuildLogRetention: &atc.BuildLogRetention{
							Builds: 0,
							Days:   1,
						},
					}, nil)

					fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)
					fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
				})

				It("should delete before that", func() {
					err := buildLogCollector.Run(context.TODO())
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))
					actualBuildIDs := fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)
					Expect(actualBuildIDs).To(ConsistOf(5))
				})
			})

			Context("when min_success_build is set", func() {
				BeforeEach(func() {
					fakeJob.ConfigReturns(atc.JobConfig{
						BuildLogRetention: &atc.BuildLogRetention{
							Builds:                 5,
							Days:                   0,
							MinimumSucceededBuilds: 2,
						},
					}, nil)

					page1 := db.Page{From: 5, Limit: 5}
					page2 := db.Page{From: 10, Limit: 5}
					page3 := db.Page{From: 15, Limit: 5}
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == page1 {
							return []db.Build{sb(9), successBuild(8), sb(7), reapedBuild(6), reapedBuild(5)}, db.Pagination{Newer: &page2}, nil
						} else if page == page2 {
							return []db.Build{sb(14), successBuild(13), sb(12), sb(11), sb(10)}, db.Pagination{Newer: &page3}, nil
						} else if page == page3 {
							return []db.Build{sb(18), sb(17), sb(16), sb(15)}, db.Pagination{}, nil
						}
						Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						return nil, db.Pagination{}, nil
					}
				})

				JustBeforeEach(func() {
					err := buildLogCollector.Run(context.TODO())
					Expect(err).NotTo(HaveOccurred())
				})

				It("should reap non success builds", func() {
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))
					actualBuildIDs := fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)
					Expect(actualBuildIDs).To(ConsistOf(7, 9, 10, 11, 12, 14, 15))

					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).Should(Not(ContainElement(5)))
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).Should(Not(ContainElement(6)))
				})

				It("should keep at least n success builds, n=MinSuccessBuilds, n=2 ", func() {
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).Should(Not(ContainElement(8)))
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).Should(Not(ContainElement(13)))
				})

				It("should update first logged build id to the earliest success build", func() {
					Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(Equal(1))
					actualNewFirstLoggedBuildID := fakeJob.UpdateFirstLoggedBuildIDArgsForCall(0)
					Expect(actualNewFirstLoggedBuildID).To(Equal(8))
				})
			})

			Context("when min_success_build equals builds", func() {
				BeforeEach(func() {
					fakeJob.ConfigReturns(atc.JobConfig{
						BuildLogRetention: &atc.BuildLogRetention{
							Builds:                 5,
							Days:                   0,
							MinimumSucceededBuilds: 5,
						},
					}, nil)

					page1 := db.Page{From: 5, Limit: 5}
					page2 := db.Page{From: 10, Limit: 5}
					page3 := db.Page{From: 15, Limit: 5}
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == page1 {
							return []db.Build{sb(9), successBuild(8), sb(7), reapedBuild(6), reapedBuild(5)}, db.Pagination{Newer: &page2}, nil
						} else if page == page2 {
							return []db.Build{sb(14), successBuild(13), successBuild(12), sb(11), successBuild(10)}, db.Pagination{Newer: &page3}, nil
						} else if page == page3 {
							return []db.Build{successBuild(18), sb(17), sb(16), successBuild(15)}, db.Pagination{}, nil
						}
						Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						return nil, db.Pagination{}, nil
					}
				})

				JustBeforeEach(func() {
					err := buildLogCollector.Run(context.TODO())
					Expect(err).NotTo(HaveOccurred())
				})

				It("should reap non success builds and success builds that exceeds min success build retained number", func() {
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))
					actualBuildIDs := fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)
					Expect(actualBuildIDs).To(ConsistOf(7, 8, 9, 11, 14, 16, 17))

					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).Should(Not(ContainElement(5)))
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).Should(Not(ContainElement(6)))
				})

				It("should keep at least n success builds, n=MinSuccessBuilds, n=5", func() {
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).Should(Not(ContainElement(10)))
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).Should(Not(ContainElement(12)))
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).Should(Not(ContainElement(13)))
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).Should(Not(ContainElement(15)))
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).Should(Not(ContainElement(18)))
				})

				It("should update first logged build id to the earliest success build", func() {
					Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(Equal(1))
					actualNewFirstLoggedBuildID := fakeJob.UpdateFirstLoggedBuildIDArgsForCall(0)
					Expect(actualNewFirstLoggedBuildID).To(Equal(10))
				})
			})
		})

		Context("when the FirstLoggedBuildID has an value", func() {
			Context("when all the logs get reaped", func() {
				var fakeJob *dbfakes.FakeJob

				BeforeEach(func() {
					fakeJob = new(dbfakes.FakeJob)
					fakeJob.NameReturns("job-1")
					fakeJob.FirstLoggedBuildIDReturns(5)
					fakeJob.ConfigReturns(atc.JobConfig{
						BuildLogRetention: &atc.BuildLogRetention{
							Days: 1,
						},
					}, nil)

					fakePipeline.JobsReturns([]db.Job{fakeJob}, nil)

					yesterday := time.Now().Add(-30 * time.Hour)

					fakeJob.BuildsReturns([]db.Build{sbTime(9, yesterday), sbTime(8, yesterday), sbTime(7, yesterday), sbTime(6, yesterday), sbTime(5, yesterday)}, db.Pagination{}, nil)
				})

				It("FirstLoggedBuildID doesn't get reset to 0", func() {
					Expect(buildLogCollector.Run(context.TODO())).NotTo(HaveOccurred())
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))
					Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).To(ConsistOf(9, 8, 7, 6, 5))
					Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(Equal(0))
				})
			})

			Context("when FirstLoggedBuildID == 1", func() {
				var fakeJob *dbfakes.FakeJob

				BeforeEach(func() {
					fakeJob = new(dbfakes.FakeJob)
					fakeJob.NameReturns("job-1")
					fakeJob.FirstLoggedBuildIDReturns(1)
					fakeJob.ConfigReturns(atc.JobConfig{
						BuildLogsToRetain: 10,
					}, nil)

					fakePipeline.JobsReturns([]db.Job{fakeJob}, nil)
				})

				Context("when we install a custom build log retention calculator", func() {
					BeforeEach(func() {
						buildLogRetainCalc = NewBuildLogRetentionCalculator(3, 3, 0, 0)

						fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
							if page == (db.Page{From: 1, Limit: 5}) {
								return []db.Build{sb(4), sb(3), sb(2), sb(1)}, db.Pagination{}, nil
							}

							Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
							return nil, db.Pagination{}, nil
						}

						fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)
						fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
					})

					It("uses build log calculator", func() {
						Expect(buildLogCollector.Run(context.TODO())).NotTo(HaveOccurred())
						Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))
						Expect(fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)).To(ConsistOf(1))
					})
				})

				Context("when getting the job builds fails", func() {
					var disaster error

					BeforeEach(func() {
						disaster = errors.New("major malfunction")

						fakeJob.BuildsReturns(nil, db.Pagination{}, disaster)
					})

					It("returns the error", func() {
						err := buildLogCollector.Run(context.TODO())
						Expect(err).To(Equal(disaster))
					})
				})
			})

		})

		Context("when the job says retain 0 builds", func() {
			var fakeJob *dbfakes.FakeJob

			BeforeEach(func() {
				fakeJob = new(dbfakes.FakeJob)
				fakeJob.NameReturns("job-1")
				fakeJob.FirstLoggedBuildIDReturns(6)
				fakeJob.ConfigReturns(atc.JobConfig{
					BuildLogsToRetain: 0,
				}, nil)
				fakeJob.TagsReturns([]string{})

				fakePipeline.JobsReturns([]db.Job{fakeJob}, nil)
			})

			It("skips the reaping step for that job", func() {
				err := buildLogCollector.Run(context.TODO())
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeJob.BuildsCallCount()).To(BeZero())
				Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(BeZero())
				Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(BeZero())
			})
		})
	})

	Context("when there is a paused pipeline", func() {
		var fakePipeline *dbfakes.FakePipeline

		BeforeEach(func() {
			fakePipeline = new(dbfakes.FakePipeline)
			fakePipeline.IDReturns(42)
			fakePipeline.PausedReturns(true)

			fakePipelineFactory.AllPipelinesReturns([]db.Pipeline{fakePipeline}, nil)
		})

		It("skips the reaping step for that pipeline", func() {
			err := buildLogCollector.Run(context.TODO())
			Expect(err).NotTo(HaveOccurred())

			Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(BeZero())
		})
	})

	Context("when getting the pipelines fails", func() {
		var disaster error

		BeforeEach(func() {
			disaster = errors.New("major malfunction")

			fakePipelineFactory.AllPipelinesReturns(nil, disaster)
		})

		It("returns the error", func() {
			err := buildLogCollector.Run(context.TODO())
			Expect(err).To(Equal(disaster))
		})
	})

})

func sb(id int) db.Build {
	build := new(dbfakes.FakeBuild)
	build.IDReturns(id)
	build.IsRunningReturns(false)
	return build
}

func sbTime(id int, end time.Time) db.Build {
	build := new(dbfakes.FakeBuild)
	build.IDReturns(id)
	build.EndTimeReturns(end)
	build.IsRunningReturns(false)
	return build
}

func sbDrained(id int, drained bool) db.Build {
	build := new(dbfakes.FakeBuild)
	build.IsDrainedReturns(drained)
	build.IDReturns(id)
	build.IsRunningReturns(false)
	return build
}

func runningBuild(id int) db.Build {
	build := new(dbfakes.FakeBuild)
	build.IDReturns(id)
	build.IsRunningReturns(true)
	return build
}

func reapedBuild(id int) db.Build {
	build := new(dbfakes.FakeBuild)
	build.IDReturns(id)
	build.ReapTimeReturns(time.Now())
	return build
}

func successBuild(id int) db.Build {
	build := new(dbfakes.FakeBuild)
	build.IDReturns(id)
	build.StatusReturns(db.BuildStatusSucceeded)
	return build
}
