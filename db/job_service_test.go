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
					Expect(err).NotTo(HaveOccurred())
					Expect(service).To(Equal(db.JobService{
						JobConfig: atc.JobConfig{},
						DBJob:     dbJob,
						DB:        fakeDB,
					}))

				})

				Context("when the GetJob lookup fails", func() {
					It("returns an error", func() {
						fakeDB.GetJobReturns(db.SavedJob{}, errors.New("disaster"))
						_, err := db.NewJobService(atc.JobConfig{}, fakeDB)
						Expect(err).To(HaveOccurred())
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
						Expect(err).NotTo(HaveOccurred())

						canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(db.Build{})
						Expect(err).NotTo(HaveOccurred())
						Expect(reason).To(Equal("job-paused"))
						Expect(canBuildBeScheduled).To(BeFalse())
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
						Expect(err).NotTo(HaveOccurred())
					})

					It("returns true if the build status is pending", func() {
						canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(db.Build{
							Status: db.StatusPending,
						})

						Expect(err).NotTo(HaveOccurred())
						Expect(reason).To(Equal("can-be-scheduled"))
						Expect(canBuildBeScheduled).To(BeTrue())
					})

					It("returns false if the build status is not pending", func() {
						canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(db.Build{
							Status: db.StatusStarted,
						})

						Expect(err).NotTo(HaveOccurred())
						Expect(reason).To(Equal("build-not-pending"))
						Expect(canBuildBeScheduled).To(BeFalse())
					})

					Context("When the job is serial", func() {
						var service db.JobService
						var dbBuild db.Build

						BeforeEach(func() {
							var err error
							service, err = db.NewJobService(atc.JobConfig{
								Serial: true,
							}, fakeDB)

							Expect(err).NotTo(HaveOccurred())
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
								Expect(err).To(HaveOccurred())
								Expect(reason).To(Equal("db-failed"))
								Expect(canBuildBeScheduled).To(BeFalse())
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
								Expect(err).NotTo(HaveOccurred())
								Expect(reason).To(Equal("max-in-flight-reached"))
								Expect(canBuildBeScheduled).To(BeFalse())
							})
						})

						Context("When no other builds are running", func() {
							BeforeEach(func() {
								fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{}, nil)
							})

							Context("When the call to get next pending build fails", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{}, false, errors.New("disaster"))
								})

								It("returns false and the error", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)
									Expect(err).To(HaveOccurred())
									Expect(reason).To(Equal("db-failed"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})

							Context("When it is not the next most pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{ID: 3}, true, nil)
								})

								It("returns false", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("not-next-most-pending"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})

							Context("When there is no pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{}, false, nil)
								})

								It("returns false", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("no-pending-build"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})

							Context("When it is the next most pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{ID: dbBuild.ID}, true, nil)
								})

								It("returns true", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)

									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("can-be-scheduled"))
									Expect(canBuildBeScheduled).To(BeTrue())
								})
							})
						})
					})

					Context("When the job has a max-in-flight of 3", func() {
						var service db.JobService
						var dbBuild db.Build

						BeforeEach(func() {
							var err error
							service, err = db.NewJobService(atc.JobConfig{
								RawMaxInFlight: 3,
							}, fakeDB)

							Expect(err).NotTo(HaveOccurred())
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
								Expect(err).To(HaveOccurred())
								Expect(reason).To(Equal("db-failed"))
								Expect(canBuildBeScheduled).To(BeFalse())
							})
						})

						Context("When 1 build is running", func() {
							BeforeEach(func() {
								fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{
									{
										Name: "Some-build",
									},
								}, nil)
							})

							Context("when the build is the next in line", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{ID: dbBuild.ID}, true, nil)
								})

								It("returns true", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)

									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("can-be-scheduled"))
									Expect(canBuildBeScheduled).To(BeTrue())
								})
							})

							Context("when the build is not next in line", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{ID: dbBuild.ID - 1}, true, nil)
								})

								It("returns false", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("not-next-most-pending"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})

							Context("When there is no pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{}, false, nil)
								})

								It("returns false", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("no-pending-build"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})
						})

						Context("When 2 builds are running", func() {
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

							Context("when the build is the next in line", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{ID: dbBuild.ID}, true, nil)
								})

								It("returns true", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)

									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("can-be-scheduled"))
									Expect(canBuildBeScheduled).To(BeTrue())
								})
							})

							Context("when the build is not next in line", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{ID: dbBuild.ID - 1}, true, nil)
								})

								It("returns false", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("not-next-most-pending"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})

							Context("When there is no pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{}, false, nil)
								})

								It("returns false", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("no-pending-build"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})
						})

						Context("When the max-in-flight is already reached", func() {
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

								Expect(err).NotTo(HaveOccurred())
								Expect(reason).To(Equal("max-in-flight-reached"))
								Expect(canBuildBeScheduled).To(BeFalse())
							})
						})

						Context("When no other builds are running", func() {
							BeforeEach(func() {
								fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{}, nil)
							})

							Context("When the call to get next pending build fails", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{}, false, errors.New("disaster"))
								})

								It("returns false and the error", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)
									Expect(err).To(HaveOccurred())
									Expect(reason).To(Equal("db-failed"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})

							Context("When it is not the next most pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{ID: 3}, true, nil)
								})

								It("returns false", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("not-next-most-pending"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})

							Context("When there is no pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{}, false, nil)
								})

								It("returns false", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("no-pending-build"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})

							Context("When it is the next most pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{ID: dbBuild.ID}, true, nil)
								})

								It("returns true", func() {
									canBuildBeScheduled, reason, err := service.CanBuildBeScheduled(dbBuild)

									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("can-be-scheduled"))
									Expect(canBuildBeScheduled).To(BeTrue())
								})
							})
						})
					})
				})
			})
		})
	}
}
