package gc_test

import (
	"errors"
	"fmt"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	. "github.com/concourse/atc/gc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildReaper", func() {
	var (
		buildReaper         BuildReaper
		fakePipelineFactory *dbfakes.FakePipelineFactory
		batchSize           int
	)

	BeforeEach(func() {
		fakePipelineFactory = new(dbfakes.FakePipelineFactory)
		batchSize = 5
	})

	JustBeforeEach(func() {
		buildReaperLogger := lagertest.NewTestLogger("test")
		buildReaper = NewBuildReaper(
			buildReaperLogger,
			fakePipelineFactory,
			batchSize,
		)
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
				err := buildReaper.Run()
				Expect(err).To(Equal(disaster))
			})
		})

		Context("when the dashboard has a job", func() {
			var fakeJob *dbfakes.FakeJob

			BeforeEach(func() {
				fakeJob = new(dbfakes.FakeJob)
				fakeJob.NameReturns("job-1")
				fakeJob.FirstLoggedBuildIDReturns(6)
				fakeJob.ConfigReturns(atc.JobConfig{
					BuildLogsToRetain: 10,
				})

				fakePipeline.JobsReturns([]db.Job{fakeJob}, nil)
			})

			Context("when there are more build logs than we can reap in this run", func() {
				BeforeEach(func() {
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == (db.Page{Limit: 10}) {
							return []db.Build{sb(25), sb(24), sb(23), sb(22), sb(21), sb(20), sb(19), sb(18), sb(17), sb(16)}, db.Pagination{}, nil
						} else if page == (db.Page{Until: 5, Limit: 5}) {
							return []db.Build{sb(10), sb(9), sb(8), sb(7), sb(6)}, db.Pagination{}, nil
						} else {
							Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						}
						return nil, db.Pagination{}, nil
					}
				})

				Context("when deleting build events and updating first logged build id succeed", func() {
					BeforeEach(func() {
						fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)
						fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
					})

					It("reaps n builds starting with FirstLoggedBuildID, n = batchSize", func() {
						err := buildReaper.Run()
						Expect(err).NotTo(HaveOccurred())

						Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))
						actualBuildIDs := fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)
						Expect(actualBuildIDs).To(ConsistOf(6, 7, 8, 9, 10))
					})

					It("updates FirstLoggedBuildID to n+1, n = latest reaped build ID", func() {
						err := buildReaper.Run()
						Expect(err).NotTo(HaveOccurred())

						Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(Equal(1))
						actualNewFirstLoggedBuildID := fakeJob.UpdateFirstLoggedBuildIDArgsForCall(0)
						Expect(actualNewFirstLoggedBuildID).To(Equal(11))
					})
				})

				Context("when deleting build events fails", func() {
					var disaster error

					BeforeEach(func() {
						disaster = errors.New("major malfunction")

						fakePipeline.DeleteBuildEventsByBuildIDsReturns(disaster)
					})

					It("returns the error", func() {
						err := buildReaper.Run()
						Expect(err).To(Equal(disaster))
					})

					It("does not update first logged build id", func() {
						buildReaper.Run()

						Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(BeZero())
					})
				})

				Context("when updating first logged build id fails", func() {
					var disaster error

					BeforeEach(func() {
						disaster = errors.New("major malfunction")

						fakeJob.UpdateFirstLoggedBuildIDReturns(disaster)
					})

					It("returns the error", func() {
						err := buildReaper.Run()
						Expect(err).To(Equal(disaster))
					})
				})
			})

			Context("when there are fewer build logs than we can reap in this run", func() {
				BeforeEach(func() {
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == (db.Page{Limit: 10}) {
							return []db.Build{sb(18), sb(17), sb(16), sb(15), sb(14), sb(13), sb(12), sb(11), sb(10), sb(9)}, db.Pagination{}, nil
						} else if page == (db.Page{Until: 5, Limit: 5}) {
							return []db.Build{sb(10), sb(9), sb(8), sb(7), sb(6)}, db.Pagination{}, nil
						} else {
							Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						}
						return nil, db.Pagination{}, nil
					}

					fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)

					fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
				})

				It("reaps n builds starting with FirstLoggedBuildID, n = batchSize", func() {
					err := buildReaper.Run()
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))
					actualBuildIDs := fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)
					Expect(actualBuildIDs).To(ConsistOf(6, 7, 8))
				})

				It("updates FirstLoggedBuildID to n+1, n = latest reaped build ID", func() {
					err := buildReaper.Run()
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(Equal(1))
					actualNewFirstLoggedBuildID := fakeJob.UpdateFirstLoggedBuildIDArgsForCall(0)
					Expect(actualNewFirstLoggedBuildID).To(Equal(9))
				})
			})

			Context("when the builds we want to reap are still running", func() {
				BeforeEach(func() {
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == (db.Page{Limit: 10}) {
							return []db.Build{sb(25), sb(24), sb(23), sb(22), sb(21), sb(20), sb(19), sb(18), sb(17), sb(16)}, db.Pagination{}, nil
						} else if page == (db.Page{Until: 5, Limit: 5}) {
							return []db.Build{
								sb(10),
								runningBuild(9),
								runningBuild(8),
								sb(7),
								sb(6),
							}, db.Pagination{}, nil
						} else {
							Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						}
						return nil, db.Pagination{}, nil
					}

					fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)

					fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
				})

				It("reaps all builds before the first running build", func() {
					err := buildReaper.Run()
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))
					actualBuildIDs := fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)
					Expect(actualBuildIDs).To(ConsistOf(6, 7))
				})

				It("updates FirstLoggedBuildID to n+1, n = latest reaped build ID", func() {
					err := buildReaper.Run()
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(Equal(1))
					actualNewFirstLoggedBuildID := fakeJob.UpdateFirstLoggedBuildIDArgsForCall(0)
					Expect(actualNewFirstLoggedBuildID).To(Equal(8))
				})
			})

			Context("when no builds need to be reaped", func() {
				BeforeEach(func() {
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == (db.Page{Limit: 10}) {
							return []db.Build{sb(12), sb(11), sb(10), sb(9), sb(8), sb(7), sb(6), sb(5), sb(4), sb(3)}, db.Pagination{}, nil
						} else if page == (db.Page{Until: 5, Limit: 5}) {
							return []db.Build{sb(10), sb(9), sb(8), sb(7), sb(6)}, db.Pagination{}, nil
						} else {
							Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						}
						return nil, db.Pagination{}, nil
					}

					fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)

					fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
				})

				It("doesn't reap any builds", func() {
					err := buildReaper.Run()
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(BeZero())
				})

				It("doesn't update FirstLoggedBuildID", func() {
					err := buildReaper.Run()
					Expect(err).NotTo(HaveOccurred())

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
					err := buildReaper.Run()
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
					err := buildReaper.Run()
					Expect(err).To(Equal(disaster))
				})
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
				})

				fakePipeline.JobsReturns([]db.Job{fakeJob}, nil)
			})

			Context("when a build of this job has build id 1", func() {
				BeforeEach(func() {
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == (db.Page{Limit: 10}) {
							return []db.Build{sb(25), sb(24), sb(23), sb(22), sb(21), sb(20), sb(19), sb(18), sb(17), sb(16)}, db.Pagination{}, nil
						} else if page == (db.Page{Until: 1, Limit: 4}) {
							return []db.Build{sb(5), sb(4), sb(3), sb(2)}, db.Pagination{}, nil
						} else if page == (db.Page{Since: 2, Limit: 1}) {
							return []db.Build{sb(1)}, db.Pagination{}, nil
						} else {
							Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						}
						return nil, db.Pagination{}, nil
					}

					fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)
					fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
				})

				It("reaps n builds starting with build 1, n = batchSize", func() {
					err := buildReaper.Run()
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))
					actualBuildIDs := fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)
					Expect(actualBuildIDs).To(ConsistOf(1, 2, 3, 4, 5))
				})

				It("updates FirstLoggedBuildID to n+1, n = latest reaped build ID", func() {
					err := buildReaper.Run()
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(Equal(1))
					actualNewFirstLoggedBuildID := fakeJob.UpdateFirstLoggedBuildIDArgsForCall(0)
					Expect(actualNewFirstLoggedBuildID).To(Equal(6))
				})

				Context("when batchSize == 1", func() {
					BeforeEach(func() {
						batchSize = 1
					})

					It("reaps n builds starting with build 1, n = batchSize", func() {
						err := buildReaper.Run()
						Expect(err).NotTo(HaveOccurred())

						Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))
						actualBuildIDs := fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)
						Expect(actualBuildIDs).To(ConsistOf(1))
					})

					It("updates FirstLoggedBuildID to n+1, n = latest reaped build ID", func() {
						err := buildReaper.Run()
						Expect(err).NotTo(HaveOccurred())

						Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(Equal(1))
						actualNewFirstLoggedBuildID := fakeJob.UpdateFirstLoggedBuildIDArgsForCall(0)
						Expect(actualNewFirstLoggedBuildID).To(Equal(2))
					})
				})
			})

			Context("when no build of this job has build id 1", func() {
				BeforeEach(func() {
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == (db.Page{Limit: 10}) {
							return []db.Build{sb(25), sb(24), sb(23), sb(22), sb(21), sb(20), sb(19), sb(18), sb(17), sb(16)}, db.Pagination{}, nil
						} else if page == (db.Page{Until: 1, Limit: 5}) {
							return []db.Build{sb(6), sb(5), sb(4), sb(3), sb(2)}, db.Pagination{}, nil
						} else if page == (db.Page{Since: 2, Limit: 1}) {
							return []db.Build{}, db.Pagination{}, nil
						} else {
							Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						}
						return nil, db.Pagination{}, nil
					}

					fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)
					fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
				})

				It("reaps n builds starting with FirstLoggedBuildID, n = batchSize", func() {
					err := buildReaper.Run()
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))
					actualBuildIDs := fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)
					Expect(actualBuildIDs).To(ConsistOf(2, 3, 4, 5, 6))
				})
			})

			Context("when getting the job builds fails", func() {
				var disaster error

				BeforeEach(func() {
					disaster = errors.New("major malfunction")

					fakeJob.BuildsReturns(nil, db.Pagination{}, disaster)
				})

				It("returns the error", func() {
					err := buildReaper.Run()
					Expect(err).To(Equal(disaster))
				})
			})
		})

		Context("when FirstLoggedBuildID == 0", func() {
			var fakeJob *dbfakes.FakeJob

			BeforeEach(func() {
				fakeJob = new(dbfakes.FakeJob)
				fakeJob.NameReturns("job-1")
				fakeJob.FirstLoggedBuildIDReturns(0)
				fakeJob.ConfigReturns(atc.JobConfig{
					BuildLogsToRetain: 10,
				})

				fakePipeline.JobsReturns([]db.Job{fakeJob}, nil)
			})

			Context("when a build of this job has build id 1", func() {
				BeforeEach(func() {
					fakeJob.BuildsStub = func(page db.Page) ([]db.Build, db.Pagination, error) {
						if page == (db.Page{Limit: 10}) {
							return []db.Build{sb(25), sb(24), sb(23), sb(22), sb(21), sb(20), sb(19), sb(18), sb(17), sb(16)}, db.Pagination{}, nil
						} else if page == (db.Page{Until: 1, Limit: 4}) {
							return []db.Build{sb(5), sb(4), sb(3), sb(2)}, db.Pagination{}, nil
						} else if page == (db.Page{Since: 2, Limit: 1}) {
							return []db.Build{sb(1)}, db.Pagination{}, nil
						} else {
							Fail(fmt.Sprintf("Builds called with unexpected argument: page=%#v", page))
						}
						return nil, db.Pagination{}, nil
					}

					fakePipeline.DeleteBuildEventsByBuildIDsReturns(nil)
					fakeJob.UpdateFirstLoggedBuildIDReturns(nil)
				})

				It("reaps n builds starting with build 1, n = batchSize", func() {
					err := buildReaper.Run()
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipeline.DeleteBuildEventsByBuildIDsCallCount()).To(Equal(1))
					actualBuildIDs := fakePipeline.DeleteBuildEventsByBuildIDsArgsForCall(0)
					Expect(actualBuildIDs).To(ConsistOf(1, 2, 3, 4, 5))
				})

				It("updates FirstLoggedBuildID to n+1, n = latest reaped build ID", func() {
					err := buildReaper.Run()
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeJob.UpdateFirstLoggedBuildIDCallCount()).To(Equal(1))
					actualNewFirstLoggedBuildID := fakeJob.UpdateFirstLoggedBuildIDArgsForCall(0)
					Expect(actualNewFirstLoggedBuildID).To(Equal(6))
				})
			})
		})

		Context("when the dashboard job says retain 0 builds", func() {
			var fakeJob *dbfakes.FakeJob

			BeforeEach(func() {
				fakeJob = new(dbfakes.FakeJob)
				fakeJob.NameReturns("job-1")
				fakeJob.FirstLoggedBuildIDReturns(6)
				fakeJob.ConfigReturns(atc.JobConfig{
					BuildLogsToRetain: 0,
				})

				fakePipeline.DashboardReturns(db.Dashboard{
					{
						Job: fakeJob,
					},
				}, atc.GroupConfigs{}, nil)
			})

			It("skips the reaping step for that job", func() {
				err := buildReaper.Run()
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
			err := buildReaper.Run()
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
			err := buildReaper.Run()
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

func runningBuild(id int) db.Build {
	build := new(dbfakes.FakeBuild)
	build.IDReturns(id)
	build.IsRunningReturns(true)
	return build
}
