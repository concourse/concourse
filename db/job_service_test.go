package db_test

import (
	"errors"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func jobService(database *dbSharedBehaviorInput) func() {
	return func() {
		Describe("Db/JobService", func() {
			var fakeDB *fakes.FakeJobServiceDB

			Describe("NewJobService", func() {
				BeforeEach(func() {
					fakeDB = new(fakes.FakeJobServiceDB)
				})

				It("sets the JobConfig and the DBJob", func() {
					dbJob := db.Job{
						Name: "a-job",
					}

					fakeDB.GetJobReturns(dbJob, nil)

					service, err := db.NewJobService(atc.JobConfig{}, fakeDB)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(service).Should(Equal(db.JobService{
						JobConfig: atc.JobConfig{},
						DBJob:     dbJob,
						DB:        fakeDB,
					}))
				})

				Context("when the GetJob lookup fails", func() {
					It("returns an error", func() {
						fakeDB.GetJobReturns(db.Job{}, errors.New("disaster"))
						_, err := db.NewJobService(atc.JobConfig{}, fakeDB)
						Ω(err).Should(HaveOccurred())
					})
				})
			})

			Describe("CanBuildBeScheduled", func() {
				BeforeEach(func() {
					fakeDB = new(fakes.FakeJobServiceDB)
				})

				Context("When the job is paused", func() {
					It("Returns false", func() {
						dbJob := db.Job{
							Paused: true,
						}

						fakeDB.GetJobReturns(dbJob, nil)
						service, err := db.NewJobService(atc.JobConfig{}, fakeDB)
						Ω(err).ShouldNot(HaveOccurred())

						canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(db.Build{})
						Ω(err).ShouldNot(HaveOccurred())
						Ω(reason).Should(Equal("job-paused"))
						Ω(canBuildBeScheduled).Should(BeFalse())
					})
				})

				Context("When the job is not paused", func() {
					var dbJob db.Job
					var service db.JobService

					BeforeEach(func() {
						var err error
						dbJob.Paused = false
						dbJob.Name = "a-job"
						fakeDB.GetJobReturns(
							dbJob,
							nil,
						)

						service, err = db.NewJobService(atc.JobConfig{}, fakeDB)
						Ω(err).ShouldNot(HaveOccurred())
					})

					It("returns true if the build status is pending", func() {
						canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(db.Build{
							Status: db.StatusPending,
						})

						Ω(err).ShouldNot(HaveOccurred())
						Ω(reason).Should(Equal("can-be-scheduled"))
						Ω(canBuildBeScheduled).Should(BeTrue())
					})

					It("returns false if the build status is not pending", func() {
						canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(db.Build{
							Status: db.StatusStarted,
						})

						Ω(err).ShouldNot(HaveOccurred())
						Ω(reason).Should(Equal("build-not-pending"))
						Ω(canBuildBeScheduled).Should(BeFalse())
					})

					Context("When the job is serial", func() {
						var service db.JobService
						var dbBuild db.Build

						BeforeEach(func() {
							var err error
							service, err = db.NewJobService(atc.JobConfig{
								Serial: true,
							}, fakeDB)

							Ω(err).ShouldNot(HaveOccurred())
							dbBuild = db.Build{
								Status: db.StatusPending,
							}
						})

						Context("When the call to get running builds throws an error", func() {
							BeforeEach(func() {
								fakeDB.GetRunningBuildsByJobReturns([]db.Build{}, errors.New("disaster"))
							})

							It("returns the error", func() {
								canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)
								Ω(err).Should(HaveOccurred())
								Ω(reason).Should(Equal("db-failed"))
								Ω(canBuildBeScheduled).Should(BeFalse())
							})
						})

						Context("When another build with the same job is running", func() {
							BeforeEach(func() {
								fakeDB.GetRunningBuildsByJobReturns([]db.Build{
									{
										Name: "Some-build",
									},
								}, nil)
							})

							It("returns false", func() {
								canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(reason).Should(Equal("other-builds-running"))
								Ω(canBuildBeScheduled).Should(BeFalse())
							})
						})

						Context("When no other builds are running", func() {
							BeforeEach(func() {
								fakeDB.GetRunningBuildsByJobReturns([]db.Build{}, nil)
							})

							Context("When the call to get next pending build fails", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildReturns(db.Build{}, []db.BuildInput{}, errors.New("disaster"))
								})

								It("returns false and the error", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)
									Ω(err).Should(HaveOccurred())
									Ω(reason).Should(Equal("db-failed"))
									Ω(canBuildBeScheduled).Should(BeFalse())
								})
							})

							Context("When it is not the next most pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildReturns(db.Build{ID: 3}, []db.BuildInput{}, nil)
								})

								It("returns false", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)
									Ω(err).ShouldNot(HaveOccurred())
									Ω(reason).Should(Equal("not-next-most-pending"))
									Ω(canBuildBeScheduled).Should(BeFalse())
								})
							})

							Context("When it is the next most pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildReturns(db.Build{ID: dbBuild.ID}, []db.BuildInput{}, nil)
								})

								It("returns true", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)

									Ω(err).ShouldNot(HaveOccurred())
									Ω(reason).Should(Equal("can-be-scheduled"))
									Ω(canBuildBeScheduled).Should(BeTrue())
								})
							})
						})
					})
				})
			})
		})
	}
}
