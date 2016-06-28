package scheduler_test

import (
	"errors"

	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/schedulerfakes"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("JobService", func() {
	var fakeDB *schedulerfakes.FakeJobServiceDB
	var fakeScanner *schedulerfakes.FakeScanner

	Describe("NewJobService", func() {
		BeforeEach(func() {
			fakeDB = new(schedulerfakes.FakeJobServiceDB)
			fakeScanner = new(schedulerfakes.FakeScanner)
		})

		It("sets the JobConfig and the DBJob", func() {
			dbJob := db.SavedJob{
				Job: db.Job{
					Name: "a-job",
				},
			}

			fakeDB.GetJobReturns(dbJob, nil)

			_, err := scheduler.NewJobService(atc.JobConfig{Name: "a-job"}, fakeDB, fakeScanner)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeDB.GetJobCallCount()).To(Equal(1))
			Expect(fakeDB.GetJobArgsForCall(0)).To(Equal("a-job"))
		})

		Context("when the GetJob lookup fails", func() {
			It("returns an error", func() {
				fakeDB.GetJobReturns(db.SavedJob{}, errors.New("disaster"))
				_, err := scheduler.NewJobService(atc.JobConfig{}, fakeDB, fakeScanner)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("CanBuildBeScheduled", func() {
		var (
			service    scheduler.JobService
			dbSavedJob db.SavedJob
			jobConfig  atc.JobConfig

			logger       *lagertest.TestLogger
			dbBuild      *dbfakes.FakeBuild
			buildPrep    db.BuildPreparation
			someVersions *algorithm.VersionsDB

			canBuildBeScheduled bool
			reason              string
			buildInputs         []db.BuildInput
			err                 error
		)

		BeforeEach(func() {
			fakeDB = new(schedulerfakes.FakeJobServiceDB)
			fakeScanner = new(schedulerfakes.FakeScanner)

			jobConfig = atc.JobConfig{
				Name: "some-job",

				Plan: atc.PlanSequence{
					{
						Get:      "some-input",
						Resource: "some-resource",
						Params:   atc.Params{"some": "params"},
						Trigger:  true,
					},
					{
						Get:      "some-other-input",
						Resource: "some-other-resource",
						Params:   atc.Params{"some": "other-params"},
						Trigger:  true,
					},
				},
			}

			dbSavedJob = db.SavedJob{
				Job: db.Job{
					Name: jobConfig.Name,
				},
			}

			dbBuild = new(dbfakes.FakeBuild)
			dbBuild.IDReturns(42)
			buildPrep = db.NewBuildPreparation(dbBuild.ID())

			logger = lagertest.NewTestLogger("test")
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
		})

		JustBeforeEach(func() {
			fakeDB.GetJobReturns(dbSavedJob, nil)
			service, err = scheduler.NewJobService(jobConfig, fakeDB, fakeScanner)
			Expect(err).NotTo(HaveOccurred())

			buildInputs, canBuildBeScheduled, reason, err = service.CanBuildBeScheduled(logger, dbBuild, buildPrep, someVersions)
		})

		Context("when the the build is marked as scheduled", func() {
			BeforeEach(func() {
				dbBuild.IsScheduledReturns(true)
			})

			It("returns true", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(reason).To(Equal("build-scheduled"))
				Expect(canBuildBeScheduled).To(BeTrue())
			})
		})

		Context("when the build has failed to schedule at least once", func() {
			BeforeEach(func() {
				buildPrep.PausedPipeline = db.BuildPreparationStatusNotBlocking
				buildPrep.PausedJob = db.BuildPreparationStatusNotBlocking
				buildPrep.Inputs["spoon"] = "too big"
				buildPrep.InputsSatisfied = db.BuildPreparationStatusBlocking
			})

			It("resets the build prep", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 1))
				returnedBuildPrep := fakeDB.UpdateBuildPreparationArgsForCall(0)
				Expect(returnedBuildPrep).To(Equal(db.NewBuildPreparation(dbBuild.ID())))
			})
		})

		Context("when the pipeline is not paused", func() {
			BeforeEach(func() {
				fakeDB.IsPausedReturns(false, nil)
			})

			It("marks the build prep paused pipeline to not blocking", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 2))
				buildPrep = fakeDB.UpdateBuildPreparationArgsForCall(1)
				Expect(buildPrep.PausedPipeline).To(Equal(db.BuildPreparationStatusNotBlocking))
			})

			Context("when the build is not marked as scheduled", func() {
				Context("when the job is paused", func() {
					BeforeEach(func() {
						dbSavedJob.Paused = true
					})

					It("returns false", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(reason).To(Equal("job-paused"))
						Expect(canBuildBeScheduled).To(BeFalse())
					})

					It("marks the build prep paused job to blocking", func() {
						Expect(err).NotTo(HaveOccurred())

						Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 3))
						buildPrep = fakeDB.UpdateBuildPreparationArgsForCall(2)
						Expect(buildPrep.PausedJob).To(Equal(db.BuildPreparationStatusBlocking))
					})
				})

				Context("when the job is not paused", func() {
					JustBeforeEach(func() {
						dbSavedJob.Paused = false
					})

					It("marks the build prep paused job to not blocking", func() {
						Expect(err).NotTo(HaveOccurred())

						Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 3))
						buildPrep = fakeDB.UpdateBuildPreparationArgsForCall(2)
						Expect(buildPrep.PausedJob).To(Equal(db.BuildPreparationStatusNotBlocking))
					})

					Context("when the build status is NOT pending", func() {
						BeforeEach(func() {
							dbBuild.StatusReturns(db.StatusStarted)
						})

						It("returns false", func() {
							Expect(err).NotTo(HaveOccurred())
							Expect(reason).To(Equal("build-not-pending"))
							Expect(canBuildBeScheduled).To(BeFalse())
						})
					})

					Context("when the build status is pending", func() {
						BeforeEach(func() {
							dbBuild.StatusReturns(db.StatusPending)
						})

						Context("when passed a versions db", func() {
							It("does not load the versions database, as it was given one", func() {
								Expect(fakeDB.LoadVersionsDBCallCount()).To(Equal(0))
							})

							It("should update the build preparation inputs with the correct state", func() {
								Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 3))

								buildPrep := fakeDB.UpdateBuildPreparationArgsForCall(2)
								Expect(buildPrep.Inputs).To(Equal(map[string]db.BuildPreparationStatus{
									"some-input":       db.BuildPreparationStatusNotBlocking,
									"some-other-input": db.BuildPreparationStatusNotBlocking,
								}))
							})
						})

						Context("when not passed a versions db", func() {
							BeforeEach(func() {
								someVersions = nil
							})

							It("correctly updates the discovery state for every input being used", func() {
								Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 8))

								Expect(fakeDB.UpdateBuildPreparationArgsForCall(2).Inputs).To(Equal(
									map[string]db.BuildPreparationStatus{
										"some-input":       db.BuildPreparationStatusUnknown,
										"some-other-input": db.BuildPreparationStatusUnknown,
									}))

								Expect(fakeDB.UpdateBuildPreparationArgsForCall(4).Inputs).To(Equal(
									map[string]db.BuildPreparationStatus{
										"some-input":       db.BuildPreparationStatusBlocking,
										"some-other-input": db.BuildPreparationStatusUnknown,
									}))

								Expect(fakeDB.UpdateBuildPreparationArgsForCall(5).Inputs).To(Equal(
									map[string]db.BuildPreparationStatus{
										"some-input":       db.BuildPreparationStatusNotBlocking,
										"some-other-input": db.BuildPreparationStatusUnknown,
									}))

								Expect(fakeDB.UpdateBuildPreparationArgsForCall(6).Inputs).To(Equal(
									map[string]db.BuildPreparationStatus{
										"some-input":       db.BuildPreparationStatusNotBlocking,
										"some-other-input": db.BuildPreparationStatusBlocking,
									}))

								Expect(fakeDB.UpdateBuildPreparationArgsForCall(7).Inputs).To(Equal(
									map[string]db.BuildPreparationStatus{
										"some-input":       db.BuildPreparationStatusNotBlocking,
										"some-other-input": db.BuildPreparationStatusNotBlocking,
									}))
							})

							Context("scanning for new versions", func() {
								It("scans for new versions for each input", func() {
									Expect(fakeScanner.ScanCallCount()).To(Equal(2))

									_, resourceName := fakeScanner.ScanArgsForCall(0)
									Expect(resourceName).To(Equal("some-resource"))

									_, resourceName = fakeScanner.ScanArgsForCall(1)
									Expect(resourceName).To(Equal("some-other-resource"))
								})

								Context("when scanning fails", func() {
									disaster := errors.New("nope")

									BeforeEach(func() {
										fakeScanner.ScanReturns(disaster)
									})

									It("errors the build", func() {
										Expect(err).To(Equal(disaster))
										Expect(reason).To(Equal("failed-to-scan"))
										Expect(canBuildBeScheduled).To(BeFalse())
									})
								})
							})

							It("attempts to access the versions dataset", func() {
								Expect(fakeDB.LoadVersionsDBCallCount()).To(Equal(1))
							})

							Context("when loading the versions dataset fails", func() {
								BeforeEach(func() {
									fakeDB.LoadVersionsDBReturns(nil, errors.New("oh no!"))
								})

								It("logs an error and returns nil", func() {
									Expect(err).To(Equal(errors.New("oh no!")))
									Expect(reason).To(Equal("failed-to-load-versions-db"))
									Expect(canBuildBeScheduled).To(BeFalse())
								})
							})
						})

						Context("able to get latest input versions", func() {
							var newInputs []db.BuildInput

							BeforeEach(func() {
								newInputs = []db.BuildInput{
									{
										Name: "some-input",
										VersionedResource: db.VersionedResource{
											Resource: "some-resource", Version: db.Version{"version": "1"},
										},
									},
									{
										Name: "some-other-input",
										VersionedResource: db.VersionedResource{
											Resource: "some-other-resource", Version: db.Version{"version": "2"},
										},
									},
								}

								fakeDB.GetNextInputVersionsReturns(newInputs, true, nil, nil)
							})

							It("can be scheduled", func() {
								Expect(err).NotTo(HaveOccurred())
								Expect(reason).To(Equal("can-be-scheduled"))
								Expect(canBuildBeScheduled).To(BeTrue())
								Expect(buildInputs).To(Equal(newInputs))
							})

							It("gets latest input versions", func() {
								Expect(fakeDB.GetNextInputVersionsCallCount()).To(Equal(1))

								versions, jobName, inputConfigs := fakeDB.GetNextInputVersionsArgsForCall(0)
								Expect(versions).To(Equal(someVersions))
								Expect(jobName).To(Equal(dbSavedJob.Name))
								Expect(inputConfigs).To(Equal([]config.JobInput{
									{
										Name:     "some-input",
										Resource: "some-resource",
										Trigger:  true,
										Params:   atc.Params{"some": "params"},
									},
									{
										Name:     "some-other-input",
										Resource: "some-other-resource",
										Trigger:  true,
										Params:   atc.Params{"some": "other-params"},
									},
								}))
							})

							It("marks the build prep's inputs statisfied as not blocking", func() {
								Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 6))
								Expect(fakeDB.UpdateBuildPreparationArgsForCall(5).InputsSatisfied).To(Equal(db.BuildPreparationStatusNotBlocking))
							})

							Context("when build prep update fails due to an error", func() {
								BeforeEach(func() {
									runCount := 0

									fakeDB.UpdateBuildPreparationStub = func(buildPrep db.BuildPreparation) error {
										if runCount == 5 {
											return errors.New("noooope")
										}
										runCount++
										return nil
									}
								})

								It("returns an error with a reason", func() {
									Expect(err).To(HaveOccurred())
									Expect(reason).To(Equal("failed-to-update-build-prep-with-inputs-satisfied"))
								})
							})

							It("marks inputs as being used for build", func() {
								Expect(fakeDB.UseInputsForBuildCallCount()).To(Equal(1))

								buildID, inputs := fakeDB.UseInputsForBuildArgsForCall(0)
								Expect(buildID).To(Equal(dbBuild.ID()))
								Expect(inputs).To(ConsistOf(newInputs))
							})

							Context("when marking inputs as being used for the build fails", func() {
								BeforeEach(func() {
									fakeDB.UseInputsForBuildReturns(errors.New("this does not compute"))
								})
								Context("due to an error", func() {
									It("logs and returns nil", func() {
										Expect(err).To(HaveOccurred())
										Expect(reason).To(Equal("failed-to-use-inputs-for-build"))
									})
								})
							})

							It("updates max in flight build prep to be blocking", func() {
								Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 7))
								Expect(fakeDB.UpdateBuildPreparationArgsForCall(6).MaxRunningBuilds).To(Equal(db.BuildPreparationStatusBlocking))
							})

							Context("when the job is serial", func() {
								BeforeEach(func() {
									jobConfig.Serial = true
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
											new(dbfakes.FakeBuild),
										}, nil)
									})

									It("returns false and the error", func() {
										Expect(err).NotTo(HaveOccurred())
										Expect(reason).To(Equal("max-in-flight-reached"))
										Expect(canBuildBeScheduled).To(BeFalse())
									})

									It("marks the build prep max running builds to blocking", func() {
										Expect(err).NotTo(HaveOccurred())

										Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 7))
										buildPrep = fakeDB.UpdateBuildPreparationArgsForCall(6)
										Expect(buildPrep.MaxRunningBuilds).To(Equal(db.BuildPreparationStatusBlocking))
									})
								})

								Context("when no other builds are running", func() {
									BeforeEach(func() {
										fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{}, nil)
									})

									Context("when the call to get next pending build fails", func() {
										BeforeEach(func() {
											fakeDB.GetNextPendingBuildBySerialGroupReturns(nil, false, errors.New("disaster"))
										})

										It("returns false and the error", func() {
											Expect(err).To(HaveOccurred())
											Expect(reason).To(Equal("db-failed"))
											Expect(canBuildBeScheduled).To(BeFalse())
										})
									})

									Context("when it is not the next most pending build", func() {
										BeforeEach(func() {
											nextPendingBuild := new(dbfakes.FakeBuild)
											nextPendingBuild.IDReturns(32)
											fakeDB.GetNextPendingBuildBySerialGroupReturns(nextPendingBuild, true, nil)
										})

										It("returns false", func() {
											Expect(err).NotTo(HaveOccurred())
											Expect(reason).To(Equal("not-next-most-pending"))
											Expect(canBuildBeScheduled).To(BeFalse())
										})
									})

									Context("when there is no pending build", func() {
										BeforeEach(func() {
											fakeDB.GetNextPendingBuildBySerialGroupReturns(nil, false, nil)
										})

										It("returns false", func() {
											Expect(err).NotTo(HaveOccurred())
											Expect(reason).To(Equal("no-pending-build"))
											Expect(canBuildBeScheduled).To(BeFalse())
										})
									})

									Context("when it is the next most pending build", func() {
										BeforeEach(func() {
											nextPendingBuild := new(dbfakes.FakeBuild)
											nextPendingBuild.IDReturns(dbBuild.ID())
											fakeDB.GetNextPendingBuildBySerialGroupReturns(nextPendingBuild, true, nil)
										})

										It("returns true", func() {
											Expect(err).NotTo(HaveOccurred())
											Expect(reason).To(Equal("can-be-scheduled"))
											Expect(canBuildBeScheduled).To(BeTrue())
										})

										It("marks the build prep max running builds to not blocking", func() {
											Expect(err).NotTo(HaveOccurred())

											Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 8))
											buildPrep = fakeDB.UpdateBuildPreparationArgsForCall(7)
											Expect(buildPrep.MaxRunningBuilds).To(Equal(db.BuildPreparationStatusNotBlocking))
										})
									})
								})
							})

							Context("when the job has a max-in-flight of 3", func() {
								BeforeEach(func() {
									jobConfig.RawMaxInFlight = 3
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
											new(dbfakes.FakeBuild),
										}, nil)
									})

									Context("when the build is the next in line", func() {
										BeforeEach(func() {
											nextPendingBuild := new(dbfakes.FakeBuild)
											nextPendingBuild.IDReturns(dbBuild.ID())
											fakeDB.GetNextPendingBuildBySerialGroupReturns(nextPendingBuild, true, nil)
										})

										It("returns true", func() {
											Expect(err).NotTo(HaveOccurred())
											Expect(reason).To(Equal("can-be-scheduled"))
											Expect(canBuildBeScheduled).To(BeTrue())
										})
									})

									Context("when the build is not next in line", func() {
										BeforeEach(func() {
											nextPendingBuild := new(dbfakes.FakeBuild)
											nextPendingBuild.IDReturns(32)
											fakeDB.GetNextPendingBuildBySerialGroupReturns(nextPendingBuild, true, nil)
										})

										It("returns false", func() {
											Expect(err).NotTo(HaveOccurred())
											Expect(reason).To(Equal("not-next-most-pending"))
											Expect(canBuildBeScheduled).To(BeFalse())
										})
									})

									Context("when there is no pending build", func() {
										BeforeEach(func() {
											fakeDB.GetNextPendingBuildBySerialGroupReturns(nil, false, nil)
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
											new(dbfakes.FakeBuild),
											new(dbfakes.FakeBuild),
										}, nil)
									})

									Context("when the build is the next in line", func() {
										BeforeEach(func() {
											nextPendingBuild := new(dbfakes.FakeBuild)
											nextPendingBuild.IDReturns(dbBuild.ID())
											fakeDB.GetNextPendingBuildBySerialGroupReturns(nextPendingBuild, true, nil)
										})

										It("returns true", func() {
											Expect(err).NotTo(HaveOccurred())
											Expect(reason).To(Equal("can-be-scheduled"))
											Expect(canBuildBeScheduled).To(BeTrue())
										})
									})

									Context("when the build is not next in line", func() {
										BeforeEach(func() {
											nextPendingBuild := new(dbfakes.FakeBuild)
											nextPendingBuild.IDReturns(32)
											fakeDB.GetNextPendingBuildBySerialGroupReturns(nextPendingBuild, true, nil)
										})

										It("returns false", func() {
											Expect(err).NotTo(HaveOccurred())
											Expect(reason).To(Equal("not-next-most-pending"))
											Expect(canBuildBeScheduled).To(BeFalse())
										})
									})

									Context("when there is no pending build", func() {
										BeforeEach(func() {
											fakeDB.GetNextPendingBuildBySerialGroupReturns(nil, false, nil)
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
											new(dbfakes.FakeBuild),
											new(dbfakes.FakeBuild),
											new(dbfakes.FakeBuild),
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
											fakeDB.GetNextPendingBuildBySerialGroupReturns(nil, false, errors.New("disaster"))
										})

										It("returns false and the error", func() {
											Expect(err).To(HaveOccurred())
											Expect(reason).To(Equal("db-failed"))
											Expect(canBuildBeScheduled).To(BeFalse())
										})
									})

									Context("when it is not the next most pending build", func() {
										BeforeEach(func() {
											nextPendingBuild := new(dbfakes.FakeBuild)
											nextPendingBuild.IDReturns(32)
											fakeDB.GetNextPendingBuildBySerialGroupReturns(nextPendingBuild, true, nil)
										})

										It("returns false", func() {
											Expect(err).NotTo(HaveOccurred())
											Expect(reason).To(Equal("not-next-most-pending"))
											Expect(canBuildBeScheduled).To(BeFalse())
										})
									})

									Context("when there is no pending build", func() {
										BeforeEach(func() {
											fakeDB.GetNextPendingBuildBySerialGroupReturns(nil, false, nil)
										})

										It("returns false", func() {
											Expect(err).NotTo(HaveOccurred())
											Expect(reason).To(Equal("no-pending-build"))
											Expect(canBuildBeScheduled).To(BeFalse())
										})
									})

									Context("when it is the next most pending build", func() {
										BeforeEach(func() {
											nextPendingBuild := new(dbfakes.FakeBuild)
											nextPendingBuild.IDReturns(dbBuild.ID())
											fakeDB.GetNextPendingBuildBySerialGroupReturns(nextPendingBuild, true, nil)
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

						Context("when getting latest input versions is not successful", func() {
							var missingInputReasons db.MissingInputReasons

							BeforeEach(func() {
								missingInputReasons = db.MissingInputReasons{
									"some-input": "some-reason",
								}
								fakeDB.GetNextInputVersionsReturns(nil, false, missingInputReasons, nil)
							})

							It("logs and returns nil", func() {
								Expect(err).ToNot(HaveOccurred())
								Expect(reason).To(Equal("no-input-versions-available"))
							})

							It("updates build preparation with missing input reasons", func() {
								latestUpdateCount := fakeDB.UpdateBuildPreparationCallCount() - 1
								buildPrep = fakeDB.UpdateBuildPreparationArgsForCall(latestUpdateCount)
								Expect(buildPrep.MissingInputReasons).To(Equal(missingInputReasons))
							})

							Context("due to an error", func() {
								BeforeEach(func() {
									fakeDB.GetNextInputVersionsReturns(nil, false, nil, errors.New("banana"))
								})

								It("logs and returns nil", func() {
									Expect(err).To(HaveOccurred())
									Expect(reason).To(Equal("failed-to-get-latest-input-versions"))
								})
							})
						})
					})
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

					Expect(fakeDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 2))
					buildPrep = fakeDB.UpdateBuildPreparationArgsForCall(1)
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
