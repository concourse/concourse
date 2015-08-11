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
					dbJob := db.SavedJob{
						Job: db.Job{
							Name: "a-job",
						},
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
						fakeDB.GetJobReturns(db.SavedJob{}, errors.New("disaster"))
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
						dbJob := db.SavedJob{
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
					var dbJob db.SavedJob
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
								fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{}, errors.New("disaster"))
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
								fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{
									{
										Name: "Some-build",
									},
								}, nil)
							})

							It("returns false and the error", func() {
								canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(reason).Should(Equal("max-in-flight-reached"))
								Ω(canBuildBeScheduled).Should(BeFalse())
							})
						})

						Context("When no other builds are running", func() {
							BeforeEach(func() {
								fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{}, nil)
							})

							Context("When the call to get next pending build fails", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{}, errors.New("disaster"))
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
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{ID: 3}, nil)
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
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{ID: dbBuild.ID}, nil)
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

					Context("When the job is serial", func() {
						var service db.JobService
						var dbBuild db.Build

						BeforeEach(func() {
							var err error
							service, err = db.NewJobService(atc.JobConfig{
								RawMaxInFlight: 3,
							}, fakeDB)

							Ω(err).ShouldNot(HaveOccurred())
							dbBuild = db.Build{
								Status: db.StatusPending,
							}
						})

						Context("When the call to get running builds throws an error", func() {
							BeforeEach(func() {
								fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{}, errors.New("disaster"))
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
								fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{
									{
										Name: "Some-build",
									},
								}, nil)
							})

							It("returns true", func() {
								canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)

								Ω(err).ShouldNot(HaveOccurred())
								Ω(reason).Should(Equal("can-be-scheduled"))
								Ω(canBuildBeScheduled).Should(BeTrue())
							})
						})

						Context("When another build with the same job is running", func() {
							BeforeEach(func() {
								fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{
									{
										Name: "Some-build",
									},
									{
										Name: "Some-other-build",
									},
								}, nil)
							})

							It("returns true", func() {
								canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)

								Ω(err).ShouldNot(HaveOccurred())
								Ω(reason).Should(Equal("can-be-scheduled"))
								Ω(canBuildBeScheduled).Should(BeTrue())
							})
						})

						Context("When another build with the same job is running", func() {
							BeforeEach(func() {
								fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{
									{
										Name: "Some-build",
									},
									{
										Name: "Some-other-build",
									},
									{
										Name: "Some-other-other-build",
									},
								}, nil)
							})

							It("returns false and the error", func() {
								canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)

								Ω(err).ShouldNot(HaveOccurred())
								Ω(reason).Should(Equal("max-in-flight-reached"))
								Ω(canBuildBeScheduled).Should(BeFalse())
							})
						})

						Context("When no other builds are running", func() {
							BeforeEach(func() {
								fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{}, nil)
							})

							Context("When the call to get next pending build fails", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{}, errors.New("disaster"))
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
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{ID: 3}, nil)
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
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{ID: dbBuild.ID}, nil)
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
