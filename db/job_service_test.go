package db_test

import (
	"errors"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Db/JobService", func() {
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
		var (
			err                 error
			service             db.JobService
			dbSavedJob          db.SavedJob
			config              atc.JobConfig
			canBuildBeScheduled bool
			reason              string
			dbBuild             db.Build
			buildPrep           db.BuildPreparation
		)

		BeforeEach(func() {
			fakeDB = new(fakes.FakeJobServiceDB)
			config = atc.JobConfig{}
			dbBuild = db.Build{}
			buildPrep = db.BuildPreparation{}
		})

		JustBeforeEach(func() {
			service, err = db.NewJobService(config, fakeDB)
			Expect(err).NotTo(HaveOccurred())

			canBuildBeScheduled, reason, err = service.CanBuildBeScheduled(dbBuild, buildPrep)
		})

		Context("when the pipeline is not paused", func() {
			BeforeEach(func() {
				fakeDB.IsPausedReturns(false, nil)
			})

			It("marks the build prep paused pipeline to not blocking", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 1))
				buildPrep = fakeDB.UpdateBuildPreparationArgsForCall(0)
				Expect(buildPrep.PausedPipeline).To(Equal(db.BuildPreparationStatusNotBlocking))
			})

			Context("when the build is not marked as scheduled", func() {
				Context("when the job is paused", func() {
					BeforeEach(func() {
						dbSavedJob.Paused = true
						fakeDB.GetJobReturns(dbSavedJob, nil)
					})

					It("returns false", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(reason).To(Equal("job-paused"))
						Expect(canBuildBeScheduled).To(BeFalse())
					})

					It("marks the build prep paused job to blocking", func() {
						Expect(err).NotTo(HaveOccurred())

						Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 2))
						buildPrep = fakeDB.UpdateBuildPreparationArgsForCall(1)
						Expect(buildPrep.PausedJob).To(Equal(db.BuildPreparationStatusBlocking))
					})
				})

				Context("when the job is not paused", func() {
					JustBeforeEach(func() {
						dbSavedJob.Paused = false
						dbSavedJob.Name = "a-job"
						fakeDB.GetJobReturns(dbSavedJob, nil)
					})

					It("marks the build prep paused job to not blocking", func() {
						Expect(err).NotTo(HaveOccurred())

						Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 2))
						buildPrep = fakeDB.UpdateBuildPreparationArgsForCall(1)
						Expect(buildPrep.PausedJob).To(Equal(db.BuildPreparationStatusNotBlocking))
					})

					Context("when the build status is pending", func() {
						BeforeEach(func() {
							dbBuild.Status = db.StatusPending
						})
						It("returns true", func() {
							Expect(err).NotTo(HaveOccurred())
							Expect(reason).To(Equal("can-be-scheduled"))
							Expect(canBuildBeScheduled).To(BeTrue())
						})
					})

					Context("when the build status is NOT pending", func() {
						BeforeEach(func() {
							dbBuild.Status = db.StatusStarted
						})
						It("returns false", func() {
							Expect(err).NotTo(HaveOccurred())
							Expect(reason).To(Equal("build-not-pending"))
							Expect(canBuildBeScheduled).To(BeFalse())
						})
					})

					Context("when the job is serial", func() {
						BeforeEach(func() {
							config.Serial = true
							dbBuild.Status = db.StatusPending
						})

						Context("when the call to get running builds throws an error", func() {
							BeforeEach(func() {
								fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{}, errors.New("disaster"))
							})

							It("returns the error", func() {
								Expect(err).To(HaveOccurred())
								Expect(reason).To(Equal("db-failed"))
								Expect(canBuildBeScheduled).To(BeFalse())
							})
						})

						Context("when another build with the same job is running", func() {
							BeforeEach(func() {
								fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{
									{
										Name: "Some-build",
									},
								}, nil)
							})

							It("returns false and the error", func() {
								Expect(err).NotTo(HaveOccurred())
								Expect(reason).To(Equal("max-in-flight-reached"))
								Expect(canBuildBeScheduled).To(BeFalse())
							})

							It("marks the build prep max running builds to blocking", func() {
								Expect(err).NotTo(HaveOccurred())

								Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 3))
								buildPrep = fakeDB.UpdateBuildPreparationArgsForCall(2)
								Expect(buildPrep.MaxRunningBuilds).To(Equal(db.BuildPreparationStatusBlocking))
							})
						})

						Context("when no other builds are running", func() {
							BeforeEach(func() {
								fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{}, nil)
							})

							Context("when the call to get next pending build fails", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{}, false, errors.New("disaster"))
								})

								It("returns false and the error", func() {
									Expect(err).To(HaveOccurred())
									Expect(reason).To(Equal("db-failed"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})

							Context("when it is not the next most pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{ID: 3}, true, nil)
								})

								It("returns false", func() {
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("not-next-most-pending"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})

							Context("when there is no pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{}, false, nil)
								})

								It("returns false", func() {
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("no-pending-build"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})

							Context("when it is the next most pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{ID: dbBuild.ID}, true, nil)
								})

								It("returns true", func() {
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("can-be-scheduled"))
									Expect(canBuildBeScheduled).To(BeTrue())
								})

								It("marks the build prep max running builds to not blocking", func() {
									Expect(err).NotTo(HaveOccurred())

									Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 3))
									buildPrep = fakeDB.UpdateBuildPreparationArgsForCall(2)
									Expect(buildPrep.MaxRunningBuilds).To(Equal(db.BuildPreparationStatusNotBlocking))
								})
							})
						})
					})

					Context("when the job has a max-in-flight of 3", func() {
						BeforeEach(func() {
							config.RawMaxInFlight = 3
							dbBuild.Status = db.StatusPending
						})

						Context("when the call to get running builds throws an error", func() {
							BeforeEach(func() {
								fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{}, errors.New("disaster"))
							})

							It("returns the error", func() {
								Expect(err).To(HaveOccurred())
								Expect(reason).To(Equal("db-failed"))
								Expect(canBuildBeScheduled).To(BeFalse())
							})
						})

						Context("when 1 build is running", func() {
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
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("not-next-most-pending"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})

							Context("when there is no pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{}, false, nil)
								})

								It("returns false", func() {
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("no-pending-build"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})
						})

						Context("when 2 builds are running", func() {
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
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("not-next-most-pending"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})

							Context("when there is no pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{}, false, nil)
								})

								It("returns false", func() {
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("no-pending-build"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})
						})

						Context("when the max-in-flight is already reached", func() {
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
								Expect(err).NotTo(HaveOccurred())
								Expect(reason).To(Equal("max-in-flight-reached"))
								Expect(canBuildBeScheduled).To(BeFalse())
							})
						})

						Context("when no other builds are running", func() {
							BeforeEach(func() {
								fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{}, nil)
							})

							Context("when the call to get next pending build fails", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{}, false, errors.New("disaster"))
								})

								It("returns false and the error", func() {
									Expect(err).To(HaveOccurred())
									Expect(reason).To(Equal("db-failed"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})

							Context("when it is not the next most pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{ID: 3}, true, nil)
								})

								It("returns false", func() {
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("not-next-most-pending"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})

							Context("when there is no pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{}, false, nil)
								})

								It("returns false", func() {
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("no-pending-build"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})

							Context("when it is the next most pending build", func() {
								BeforeEach(func() {
									fakeDB.GetNextPendingBuildBySerialGroupReturns(db.Build{ID: dbBuild.ID}, true, nil)
								})

								It("returns true", func() {
									Expect(err).NotTo(HaveOccurred())
									Expect(reason).To(Equal("can-be-scheduled"))
									Expect(canBuildBeScheduled).To(BeTrue())
								})
							})
						})
					})
				})
			})

			Context("when the the build is marked as scheduled", func() {
				BeforeEach(func() {
					dbBuild.Scheduled = true
				})

				It("returns true", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(reason).To(Equal("build-scheduled"))
					Expect(canBuildBeScheduled).To(BeTrue())
				})

				It("marks the build prep paused pipeline and max running builds to not blocking", func() {
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 2))
					buildPrep = fakeDB.UpdateBuildPreparationArgsForCall(1)
					Expect(buildPrep.PausedPipeline).To(Equal(db.BuildPreparationStatusNotBlocking))
					Expect(buildPrep.MaxRunningBuilds).To(Equal(db.BuildPreparationStatusNotBlocking))
				})
			})
		})

		Context("when the pipeline is paused", func() {
			Context("when IsPaused returns true", func() {
				BeforeEach(func() {
					fakeDB.IsPausedReturns(true, nil)
				})

				It("returns false", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(reason).To(Equal("pipeline-paused"))
					Expect(canBuildBeScheduled).To(BeFalse())
				})

				It("marks the build prep paused pipeline to blocking", func() {
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 1))
					buildPrep = fakeDB.UpdateBuildPreparationArgsForCall(0)
					Expect(buildPrep.PausedPipeline).To(Equal(db.BuildPreparationStatusBlocking))
				})
			})

			Context("when IsPaused returns an error", func() {
				BeforeEach(func() {
					fakeDB.IsPausedReturns(false, errors.New("OMFG MY BFF JILL"))
				})

				It("returns an error", func() {
					Expect(err).To(HaveOccurred())
					Expect(reason).To(Equal("pause-pipeline-db-failed"))
					Expect(canBuildBeScheduled).To(BeFalse())
				})
			})
		})
	})
})
