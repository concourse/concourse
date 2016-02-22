package scheduler_test

import (
	"errors"

	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	dbfakes "github.com/concourse/atc/db/fakes"
	"github.com/concourse/atc/engine"
	enginefakes "github.com/concourse/atc/engine/fakes"
	. "github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/fakes"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Scheduler", func() {
	var (
		fakePipelineDB *fakes.FakePipelineDB
		fakeBuildsDB   *fakes.FakeBuildsDB
		factory        *fakes.FakeBuildFactory
		fakeEngine     *enginefakes.FakeEngine
		fakeScanner    *fakes.FakeScanner

		lease *dbfakes.FakeLease

		createdPlan atc.Plan

		job           atc.JobConfig
		resources     atc.ResourceConfigs
		resourceTypes atc.ResourceTypes

		scheduler *Scheduler

		someVersions *algorithm.VersionsDB

		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakePipelineDB = new(fakes.FakePipelineDB)
		fakeBuildsDB = new(fakes.FakeBuildsDB)
		factory = new(fakes.FakeBuildFactory)
		fakeEngine = new(enginefakes.FakeEngine)
		fakeScanner = new(fakes.FakeScanner)

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

		createdPlan = atc.Plan{
			Task: &atc.TaskPlan{
				Config: &atc.TaskConfig{
					Run: atc.TaskRunConfig{Path: "some-task"},
				},
			},
		}

		factory.CreateReturns(createdPlan, nil)

		scheduler = &Scheduler{
			PipelineDB: fakePipelineDB,
			BuildsDB:   fakeBuildsDB,
			Factory:    factory,
			Engine:     fakeEngine,
			Scanner:    fakeScanner,
		}

		logger = lagertest.NewTestLogger("test")

		job = atc.JobConfig{
			Name: "some-job",

			Serial: true,

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

		resources = atc.ResourceConfigs{
			{
				Name:   "some-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-resource"},
			},
			{
				Name:   "some-other-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-other-resource"},
			},
			{
				Name:   "some-dependant-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-dependant-resource"},
			},
			{
				Name:   "some-output-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-output-resource"},
			},
			{
				Name:   "some-resource-with-longer-name",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-resource-with-longer-name"},
			},
			{
				Name:   "some-named-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-named-resource"},
			},
		}

		resourceTypes = atc.ResourceTypes{
			{
				Name:   "some-custom-resource",
				Type:   "custom-type",
				Source: atc.Source{"custom": "source"},
			},
		}

		lease = new(dbfakes.FakeLease)
		fakeBuildsDB.LeaseBuildSchedulingReturns(lease, true, nil)
	})

	Describe("BuildLatestInputs", func() {
		Context("when no inputs are available", func() {
			BeforeEach(func() {
				fakePipelineDB.GetLatestInputVersionsReturns(nil, false, nil)
			})

			It("returns no error", func() {
				err := scheduler.BuildLatestInputs(logger, someVersions, job, resources, resourceTypes)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not trigger a build", func() {
				scheduler.BuildLatestInputs(logger, someVersions, job, resources, resourceTypes)
				Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
			})
		})

		Context("when the inputs cannot be determined", func() {
			disaster := errors.New("oh no!")
			var err error

			BeforeEach(func() {
				fakePipelineDB.GetLatestInputVersionsReturns(nil, false, disaster)
				err = scheduler.BuildLatestInputs(logger, someVersions, job, resources, resourceTypes)
			})

			It("returns the error", func() {
				Expect(err).To(Equal(disaster))
			})

			It("does not trigger a build", func() {
				Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
			})
		})

		Context("when the job has no inputs", func() {
			BeforeEach(func() {
				job.Plan = atc.PlanSequence{}
				err := scheduler.BuildLatestInputs(logger, someVersions, job, resources, resourceTypes)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not try to fetch inputs from the database", func() {
				Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
			})
		})

		Context("when versions are found", func() {
			var newInputs []db.BuildInput
			var err error

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
				fakePipelineDB.GetLatestInputVersionsReturns(newInputs, true, nil)
			})

			JustBeforeEach(func() {
				err = scheduler.BuildLatestInputs(logger, someVersions, job, resources, resourceTypes)
			})

			Context("loading versions db", func() {
				BeforeEach(func() {
					pendingBuild := db.Build{
						Status: db.StatusPending,
					}
					buildPrep := db.BuildPreparation{
						Inputs: map[string]db.BuildPreparationStatus{},
					}

					fakeBuildsDB.GetBuildPreparationReturns(buildPrep, true, nil)
					fakePipelineDB.CreateJobBuildForCandidateInputsReturns(pendingBuild, true, nil)
					fakePipelineDB.GetNextPendingBuildReturns(pendingBuild, true, nil)
					fakePipelineDB.GetNextPendingBuildBySerialGroupReturns(pendingBuild, true, nil)
					fakePipelineDB.UpdateBuildToScheduledReturns(true, nil)
				})

				It("does not happen", func() {
					Expect(fakePipelineDB.LoadVersionsDBCallCount()).To(Equal(0))
				})
			})

			It("checks if they are already used for a build", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakePipelineDB.GetLatestInputVersionsCallCount()).To(Equal(1))
				versions, jobName, inputs := fakePipelineDB.GetLatestInputVersionsArgsForCall(0)
				Expect(versions).To(Equal(someVersions))
				Expect(jobName).To(Equal(job.Name))
				Expect(inputs).To(Equal([]config.JobInput{
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

				Expect(fakePipelineDB.GetJobBuildForInputsCallCount()).To(Equal(1))

				checkedJob, checkedInputs := fakePipelineDB.GetJobBuildForInputsArgsForCall(0)
				Expect(checkedJob).To(Equal("some-job"))
				Expect(checkedInputs).To(ConsistOf(newInputs))
			})

			Context("and the job has inputs configured to not trigger when they change", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, atc.PlanConfig{
						Get:     "some-non-triggering-resource",
						Trigger: false,
					})

					foundInputsWithCheck := append(
						newInputs,
						db.BuildInput{
							Name: "some-non-triggering-resource",
							VersionedResource: db.VersionedResource{
								Resource: "some-non-triggering-resource",
								Version:  db.Version{"version": "3"},
							},
						},
					)

					fakePipelineDB.GetLatestInputVersionsReturns(foundInputsWithCheck, true, nil)
				})

				It("excludes them from the inputs when checking for a build", func() {
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipelineDB.GetJobBuildForInputsCallCount()).To(Equal(1))

					checkedJob, checkedInputs := fakePipelineDB.GetJobBuildForInputsArgsForCall(0)
					Expect(checkedJob).To(Equal("some-job"))
					Expect(checkedInputs).To(Equal(newInputs))
				})
			})

			Context("and all inputs are configured not to trigger", func() {
				BeforeEach(func() {
					for i, c := range job.Plan {
						noTrigger := c
						noTrigger.Trigger = false

						job.Plan[i] = noTrigger
					}
				})

				It("does not check for builds for the inputs", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(fakePipelineDB.GetJobBuildForInputsCallCount()).To(Equal(0))
				})

				It("does not create a build", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(fakePipelineDB.CreateJobBuildForCandidateInputsCallCount()).To(Equal(0))
				})

				It("does not trigger a build", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
				})
			})

			Context("when latest inputs are not already used for a build", func() {
				BeforeEach(func() {
					fakePipelineDB.GetJobBuildForInputsReturns(db.Build{}, false, nil)
				})

				It("creates a build with the found inputs", func() {
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipelineDB.CreateJobBuildForCandidateInputsCallCount()).To(Equal(1))
					buildJob := fakePipelineDB.CreateJobBuildForCandidateInputsArgsForCall(0)
					Expect(buildJob).To(Equal("some-job"))
				})

				Context("when creating the build fails", func() {
					disaster := errors.New("oh no!")

					BeforeEach(func() {
						fakePipelineDB.CreateJobBuildForCandidateInputsReturns(db.Build{}, false, disaster)
					})

					It("returns the error", func() {
						Expect(err).To(Equal(disaster))
					})

					It("does not start a build", func() {
						scheduler.BuildLatestInputs(logger, someVersions, job, resources, resourceTypes)
						Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
					})
				})

				Context("when we do not create the build because one is already pending", func() {
					BeforeEach(func() {
						fakePipelineDB.CreateJobBuildForCandidateInputsReturns(db.Build{}, false, nil)
					})

					It("exits without error", func() {
						Expect(err).NotTo(HaveOccurred())
					})

					It("does not start a build", func() {
						scheduler.BuildLatestInputs(logger, someVersions, job, resources, resourceTypes)
						Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
					})
				})
			})

			Context("when they are already used for a build", func() {
				BeforeEach(func() {
					fakePipelineDB.GetJobBuildForInputsReturns(db.Build{ID: 128, Name: "42"}, true, nil)
				})

				It("does not enqueue or trigger a build", func() {
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipelineDB.CreateJobBuildForCandidateInputsCallCount()).To(Equal(0))
					Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
				})
			})

			Context("when we cannot determine if they are already used for a build", func() {
				disaster := errors.New("db fell over")

				BeforeEach(func() {
					fakePipelineDB.GetJobBuildForInputsReturns(db.Build{}, false, disaster)
				})

				It("does not enqueue or a build", func() {
					Expect(err).To(Equal(disaster))

					Expect(fakePipelineDB.CreateJobBuildForCandidateInputsCallCount()).To(Equal(0))
					Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
				})
			})
		})
	})

	Describe("TryNextPendingBuild", func() {

		JustBeforeEach(func() {
			scheduler.TryNextPendingBuild(logger, someVersions, job, resources, resourceTypes).Wait()
		})

		Context("when a pending build is found", func() {
			var pendingBuild db.Build

			BeforeEach(func() {
				pendingBuild = db.Build{
					ID:     128,
					Name:   "42",
					Status: db.StatusPending,
				}

				fakePipelineDB.GetNextPendingBuildReturns(pendingBuild, true, nil)
				buildPrep := db.BuildPreparation{
					Inputs: map[string]db.BuildPreparationStatus{},
				}

				fakeBuildsDB.GetBuildPreparationReturns(buildPrep, true, nil)
				fakePipelineDB.CreateJobBuildReturns(pendingBuild, nil)
				fakePipelineDB.GetNextPendingBuildBySerialGroupReturns(pendingBuild, true, nil)
				fakePipelineDB.UpdateBuildToScheduledReturns(true, nil)
			})

			It("schedules the build", func() {
				Expect(fakePipelineDB.GetNextPendingBuildCallCount()).To(Equal(1))
			})

			It("does not load the versions database, as it was given one", func() {
				Expect(fakePipelineDB.LoadVersionsDBCallCount()).To(Equal(0))
			})
		})

		Context("when a pending build is not found", func() {
			BeforeEach(func() {
				fakePipelineDB.GetNextPendingBuildReturns(db.Build{}, false, nil)
			})

			It("does not start a build", func() {
				scheduler.TryNextPendingBuild(logger, someVersions, job, resources, resourceTypes)
				Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
			})
		})

		Context("when getting the next pending build fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakePipelineDB.GetNextPendingBuildReturns(db.Build{}, false, disaster)
			})

			It("does not start a build", func() {
				scheduler.TryNextPendingBuild(logger, someVersions, job, resources, resourceTypes)
				Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
			})
		})
	})

	Describe("TriggerImmediately", func() {
		BeforeEach(func() {
			dbBuild := db.Build{
				Status: db.StatusPending,
			}
			buildPrep := db.BuildPreparation{
				Inputs: map[string]db.BuildPreparationStatus{},
			}

			fakeBuildsDB.GetBuildPreparationReturns(buildPrep, true, nil)
			fakePipelineDB.CreateJobBuildReturns(dbBuild, nil)
			fakePipelineDB.GetNextPendingBuildBySerialGroupReturns(dbBuild, true, nil)
			fakePipelineDB.UpdateBuildToScheduledReturns(true, nil)
		})

		It("creates a build without any specific inputs", func() {
			_, wg, err := scheduler.TriggerImmediately(logger, job, resources, resourceTypes)
			Expect(err).NotTo(HaveOccurred())

			wg.Wait()

			Expect(fakePipelineDB.CreateJobBuildCallCount()).To(Equal(1))

			jobName := fakePipelineDB.CreateJobBuildArgsForCall(0)
			Expect(jobName).To(Equal("some-job"))

			Expect(fakePipelineDB.LoadVersionsDBCallCount()).To(Equal(1))
		})

		Context("when creating the build fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakePipelineDB.CreateJobBuildReturns(db.Build{}, disaster)
			})

			It("returns the error", func() {
				_, _, err := scheduler.TriggerImmediately(logger, job, resources, resourceTypes)
				Expect(err).To(Equal(disaster))
			})

			It("does not start a build", func() {
				scheduler.TriggerImmediately(logger, job, resources, resourceTypes)
				Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
			})
		})
	})

	Describe("ScheduleAndResumePendingBuild", func() {
		var (
			build          db.Build
			engineBuild    engine.Build
			fakeJobService *fakes.FakeJobService
		)

		BeforeEach(func() {
			fakeJobService = new(fakes.FakeJobService)

			build = db.Build{
				ID: 123,
			}
		})

		JustBeforeEach(func() {
			engineBuild = scheduler.ScheduleAndResumePendingBuild(logger, someVersions, build, job, resources, resourceTypes, fakeJobService)
		})

		Context("when the lease is aquired", func() {
			BeforeEach(func() {
				fakeBuildsDB.LeaseBuildSchedulingReturns(lease, true, nil)

			})
			AfterEach(func() {
				Expect(lease.BreakCallCount()).To(Equal(1))
			})

			Context("when build prep can be acquired", func() {
				var buildPrep db.BuildPreparation

				BeforeEach(func() {
					buildPrep = db.BuildPreparation{
						BuildID: build.ID,
						Inputs:  map[string]db.BuildPreparationStatus{},
					}

					fakeBuildsDB.GetBuildPreparationReturns(buildPrep, true, nil)
				})

				Context("when build can be scheduled", func() {
					BeforeEach(func() {
						fakeJobService.CanBuildBeScheduledReturns(true, "yep", nil)
					})

					Context("UpdateBuildToSchedule fails to update build", func() {
						BeforeEach(func() {
							fakePipelineDB.UpdateBuildToScheduledReturns(false, errors.New("po-tate-toe"))
						})

						It("logs and returns nil", func() {
							Expect(engineBuild).To(BeNil())
							Expect(logger).To(gbytes.Say("failed-to-update-build-to-scheduled"))
						})
					})

					Context("UpdateBuildToSchedule doesn't update a build", func() {
						BeforeEach(func() {
							fakePipelineDB.UpdateBuildToScheduledReturns(false, nil)
						})

						It("logs and returns nil", func() {
							Expect(engineBuild).To(BeNil())
							Expect(logger).To(gbytes.Say("unable-to-update-build-to-scheduled"))
						})
					})

					Context("when the build is successfully marked as scheduled", func() {
						BeforeEach(func() {
							fakePipelineDB.UpdateBuildToScheduledReturns(true, nil)
						})

						Context("when passed a versions db", func() {
							It("does not load the versions database, as it was given one", func() {
								Expect(fakePipelineDB.LoadVersionsDBCallCount()).To(Equal(0))
							})

							It("should update the build preparation inputs with the correct state", func() {
								Expect(fakeBuildsDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 1))

								buildPrep := fakeBuildsDB.UpdateBuildPreparationArgsForCall(0)
								Expect(buildPrep.Inputs).To(Equal(map[string]db.BuildPreparationStatus{
									"some-input":       db.BuildPreparationStatusNotBlocking,
									"some-other-input": db.BuildPreparationStatusNotBlocking,
								}))

								Expect(buildPrep.InputsSatisfied).To(Equal(db.BuildPreparationStatusBlocking))
							})
						})

						Context("when not passed a versions db", func() {
							BeforeEach(func() {
								someVersions = nil
							})

							It("correctly updates the discovery state for every input being used", func() {
								Expect(fakeBuildsDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 5))

								for i := 0; i < 4; i++ {
									Expect(fakeBuildsDB.UpdateBuildPreparationArgsForCall(i).InputsSatisfied).To(Equal(db.BuildPreparationStatusBlocking))
								}

								Expect(fakeBuildsDB.UpdateBuildPreparationArgsForCall(0).Inputs).To(Equal(
									map[string]db.BuildPreparationStatus{
										"some-input":       db.BuildPreparationStatusUnknown,
										"some-other-input": db.BuildPreparationStatusUnknown,
									}))

								Expect(fakeBuildsDB.UpdateBuildPreparationArgsForCall(1).Inputs).To(Equal(
									map[string]db.BuildPreparationStatus{
										"some-input":       db.BuildPreparationStatusBlocking,
										"some-other-input": db.BuildPreparationStatusUnknown,
									}))

								Expect(fakeBuildsDB.UpdateBuildPreparationArgsForCall(2).Inputs).To(Equal(
									map[string]db.BuildPreparationStatus{
										"some-input":       db.BuildPreparationStatusNotBlocking,
										"some-other-input": db.BuildPreparationStatusUnknown,
									}))

								Expect(fakeBuildsDB.UpdateBuildPreparationArgsForCall(3).Inputs).To(Equal(
									map[string]db.BuildPreparationStatus{
										"some-input":       db.BuildPreparationStatusNotBlocking,
										"some-other-input": db.BuildPreparationStatusBlocking,
									}))

								Expect(fakeBuildsDB.UpdateBuildPreparationArgsForCall(4).Inputs).To(Equal(
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
										Expect(fakeBuildsDB.ErrorBuildCallCount()).To(Equal(1))

										buildID, err := fakeBuildsDB.ErrorBuildArgsForCall(0)
										Expect(buildID).To(Equal(123))
										Expect(err).To(Equal(disaster))
									})
								})
							})

							It("attempts to access the versions dataset", func() {
								Expect(fakePipelineDB.LoadVersionsDBCallCount()).To(Equal(1))
							})

							Context("when loading the versions dataset fails", func() {
								BeforeEach(func() {
									fakePipelineDB.LoadVersionsDBReturns(nil, errors.New("oh no!"))
								})

								It("does not error the build, as it may have been an ephemeral database issue", func() {
									Expect(fakeBuildsDB.ErrorBuildCallCount()).To(Equal(0))
								})

								Context("due to an error", func() {
									It("logs an error and returns nil", func() {
										Expect(engineBuild).To(BeNil())
										Expect(logger).To(gbytes.Say("failed"))
									})
								})
							})
						})

						Context("getting latest input versions", func() {
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

								fakePipelineDB.GetLatestInputVersionsReturns(newInputs, true, nil)
							})

							It("gets latest input versions", func() {
								Expect(fakePipelineDB.GetLatestInputVersionsCallCount()).To(Equal(1))

								versions, jobName, inputConfigs := fakePipelineDB.GetLatestInputVersionsArgsForCall(0)
								Expect(versions).To(Equal(someVersions))
								Expect(jobName).To(Equal(job.Name))
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

							Context("when latest input versions are successfully acquired", func() {
								BeforeEach(func() {

								})

								It("marks the build prep's inputs statisfied as not blocking", func() {
									Expect(fakeBuildsDB.UpdateBuildPreparationCallCount()).To(BeNumerically(">=", 2))
									Expect(fakeBuildsDB.UpdateBuildPreparationArgsForCall(1).InputsSatisfied).To(Equal(db.BuildPreparationStatusNotBlocking))
								})

								Context("when build prep update fails due to an error", func() {
									BeforeEach(func() {
										ranOnce := false

										fakeBuildsDB.UpdateBuildPreparationStub = func(buildPrep db.BuildPreparation) error {
											if ranOnce {
												return errors.New("noooope")
											}
											ranOnce = true
											return nil
										}
									})

									It("logs and returns nil", func() {
										Expect(engineBuild).To(BeNil())
										Expect(logger).To(gbytes.Say("failed-to-update-build-prep-with-inputs-satisfied"))
									})
								})

								It("marks inputs as being used for build", func() {
									Expect(fakePipelineDB.UseInputsForBuildCallCount()).To(Equal(1))

									buildID, inputs := fakePipelineDB.UseInputsForBuildArgsForCall(0)
									Expect(buildID).To(Equal(build.ID))
									Expect(inputs).To(ConsistOf(newInputs))
								})

								Context("when marking inputs as being used for the build fails", func() {
									BeforeEach(func() {
										fakePipelineDB.UseInputsForBuildReturns(errors.New("this does not compute"))
									})
									Context("due to an error", func() {
										It("logs and returns nil", func() {
											Expect(engineBuild).To(BeNil())
											Expect(logger).To(gbytes.Say("failed-to-use-inputs-for-build"))
										})
									})
								})

								It("creates a plan", func() {
									Expect(factory.CreateCallCount()).To(Equal(1))

									passedJob, passedResources, passedResourceTypes, passedInputs := factory.CreateArgsForCall(0)
									Expect(passedJob).To(Equal(job))
									Expect(passedResources).To(Equal(resources))
									Expect(passedResourceTypes).To(Equal(resourceTypes))
									Expect(passedInputs).To(ConsistOf(passedInputs))
								})

								Context("when making a plan for the build fails due to an error", func() {
									BeforeEach(func() {
										factory.CreateReturns(atc.Plan{}, errors.New("to err is human"))
									})

									It("marks the build as finished with an errored status and returns nil", func() {
										Expect(fakeBuildsDB.FinishBuildCallCount()).To(Equal(1))

										buildID, status := fakeBuildsDB.FinishBuildArgsForCall(0)
										Expect(buildID).To(Equal(build.ID))
										Expect(status).To(Equal(db.StatusErrored))
									})

									Context("when updating the builds status errors", func() {
										BeforeEach(func() {
											fakeBuildsDB.FinishBuildReturns(errors.New("but to really foul up requires a computer"))
										})

										It("logs and returns nil", func() {
											Expect(engineBuild).To(BeNil())
											Expect(logger).To(gbytes.Say("failed-to-mark-build-as-errored"))
										})
									})
								})

								Context("when the plan is created", func() {
									var plan atc.Plan
									var fakeEngineBuild *enginefakes.FakeBuild

									BeforeEach(func() {
										plan = atc.Plan{}
										factory.CreateReturns(plan, nil)

										fakeEngineBuild = new(enginefakes.FakeBuild)

										fakeEngine.CreateBuildReturns(fakeEngineBuild, nil)
									})

									It("tells the engine to create the build", func() {
										Expect(fakeEngine.CreateBuildCallCount()).To(Equal(1))

										_, passedBuild, passedPlan := fakeEngine.CreateBuildArgsForCall(0)
										Expect(passedBuild).To(Equal(build))
										Expect(passedPlan).To(Equal(plan))
									})

									It("returns back created build", func() {
										Expect(engineBuild).To(Equal(fakeEngineBuild))
									})

									It("calls Resume() on the created build", func() {
										Expect(logger).To(gbytes.Say("building"))
										Eventually(fakeEngineBuild.ResumeCallCount).Should(Equal(1))
									})

									Context("when the engine fails to create the build due to an error", func() {
										BeforeEach(func() {
											fakeEngine.CreateBuildReturns(nil, errors.New("no engine 4 u"))
										})

										It("logs and returns nil", func() {
											Expect(engineBuild).To(BeNil())
											Expect(logger).To(gbytes.Say("failed-to-create-build"))
										})
									})
								})
							})

							Context("when getting latest input versions is not successful", func() {
								BeforeEach(func() {
									fakePipelineDB.GetLatestInputVersionsReturns(newInputs, false, nil)
								})

								It("logs and returns nil", func() {
									Expect(engineBuild).To(BeNil())
									Expect(logger).To(gbytes.Say("no-input-versions-available"))
								})

								Context("due to an error", func() {
									BeforeEach(func() {
										fakePipelineDB.GetLatestInputVersionsReturns(newInputs, false, errors.New("banana"))
									})

									It("logs and returns nil", func() {
										Expect(engineBuild).To(BeNil())
										Expect(logger).To(gbytes.Say("failed-to-get-latest-input-versions"))
									})
								})
							})
						})

					})
				})

				Context("when build can NOT be scheduled", func() {
					BeforeEach(func() {
						fakeJobService.CanBuildBeScheduledReturns(false, "nope", nil)
					})

					It("logs and returns nil", func() {
						Expect(engineBuild).To(BeNil())
						Expect(logger).To(gbytes.Say("build-could-not-be-scheduled"))
					})

					Context("due to an error", func() {
						BeforeEach(func() {
							fakeJobService.CanBuildBeScheduledReturns(false, "db-nope", errors.New("ermagersh errorz"))
						})

						It("logs and returns nil", func() {
							Expect(engineBuild).To(BeNil())
							Expect(logger).To(gbytes.Say("failed-to-schedule-build"))
						})
					})
				})
			})

			Context("when build prep cannot be acquired", func() {
				BeforeEach(func() {
					fakeBuildsDB.GetBuildPreparationReturns(db.BuildPreparation{}, false, nil)
				})

				It("logs and returns nil", func() {
					Expect(engineBuild).To(BeNil())
					Expect(logger).To(gbytes.Say("failed-to-find-build-prep"))
				})

				Context("due to an error", func() {
					BeforeEach(func() {
						fakeBuildsDB.GetBuildPreparationReturns(db.BuildPreparation{}, false, errors.New("ermagersh an error"))
					})

					It("logs and returns nil", func() {
						Expect(engineBuild).To(BeNil())
						Expect(logger).To(gbytes.Say("failed-to-get-build-prep"))
					})
				})
			})
		})

		Context("when the lease is not aquired", func() {
			BeforeEach(func() {
				fakeBuildsDB.LeaseBuildSchedulingReturns(nil, false, nil)
			})

			It("returns nil", func() {
				Expect(engineBuild).To(BeNil())
			})

			Context("due to an error", func() {
				BeforeEach(func() {
					fakeBuildsDB.LeaseBuildSchedulingReturns(nil, false, errors.New("i screwed up boss"))
				})

				It("logs and returns nil", func() {
					Expect(engineBuild).To(BeNil())
					Expect(logger).To(gbytes.Say("failed-to-get-lease"))
				})
			})
		})
	})
})
