package scheduler_test

import (
	"errors"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	dbfakes "github.com/concourse/atc/db/fakes"
	enginefakes "github.com/concourse/atc/engine/fakes"
	. "github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/fakes"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

		job       atc.JobConfig
		resources atc.ResourceConfigs

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

		lease = new(dbfakes.FakeLease)
		fakeBuildsDB.LeaseBuildSchedulingReturns(lease, true, nil)
	})

	Describe("BuildLatestInputs", func() {
		Context("when no inputs are available", func() {
			BeforeEach(func() {
				fakePipelineDB.GetLatestInputVersionsReturns(nil, false, nil)
			})

			It("returns no error", func() {
				err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not trigger a build", func() {
				scheduler.BuildLatestInputs(logger, someVersions, job, resources)
				Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
			})
		})

		Context("when the inputs cannot be deteremined", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakePipelineDB.GetLatestInputVersionsReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
				Expect(err).To(Equal(disaster))
			})

			It("does not trigger a build", func() {
				scheduler.BuildLatestInputs(logger, someVersions, job, resources)
				Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
			})
		})

		Context("when the job has no inputs", func() {
			BeforeEach(func() {
				job.Plan = atc.PlanSequence{}
			})

			It("succeeds", func() {
				err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not load the versions database, as it was given one", func() {
				scheduler.BuildLatestInputs(logger, someVersions, job, resources)

				Expect(fakePipelineDB.LoadVersionsDBCallCount()).To(Equal(0))
			})

			It("does not try to fetch inputs from the database", func() {
				scheduler.BuildLatestInputs(logger, someVersions, job, resources)

				Expect(fakePipelineDB.GetLatestInputVersionsCallCount()).To(BeZero())
			})

			It("does not trigger a build", func() {
				scheduler.BuildLatestInputs(logger, someVersions, job, resources)

				Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
			})
		})

		Context("when versions are found", func() {
			newInputs := []db.BuildInput{
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

			BeforeEach(func() {
				fakePipelineDB.GetLatestInputVersionsReturns(newInputs, true, nil)
			})

			It("does not load the versions database, as it was given one", func() {
				scheduler.BuildLatestInputs(logger, someVersions, job, resources)

				Expect(fakePipelineDB.LoadVersionsDBCallCount()).To(Equal(0))
			})

			It("checks if they are already used for a build", func() {
				err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
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
					err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
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
					err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipelineDB.GetJobBuildForInputsCallCount()).To(Equal(0))
				})

				It("does not create a build", func() {
					err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipelineDB.CreateJobBuildForCandidateInputsCallCount()).To(Equal(0))
				})

				It("does not trigger a build", func() {
					err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
				})
			})

			Context("when they are not used for a build", func() {
				BeforeEach(func() {
					fakePipelineDB.GetJobBuildForInputsReturns(db.Build{}, false, nil)
				})

				It("creates a build with the found inputs", func() {
					err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipelineDB.CreateJobBuildForCandidateInputsCallCount()).To(Equal(1))
					buildJob := fakePipelineDB.CreateJobBuildForCandidateInputsArgsForCall(0)
					Expect(buildJob).To(Equal("some-job"))
				})

				Context("when creating the build succeeds", func() {
					BeforeEach(func() {
						fakePipelineDB.CreateJobBuildForCandidateInputsReturns(
							db.Build{
								ID:   128,
								Name: "42",
							},
							true,
							nil,
						)

						fakePipelineDB.GetNextPendingBuildReturns(
							db.Build{
								ID:   128,
								Name: "42",
							},
							true,
							nil,
						)
					})

					Context("and it can be scheduled", func() {
						BeforeEach(func() {
							fakePipelineDB.ScheduleBuildReturns(true, nil)
							fakeBuildsDB.GetBuildPreparationReturns(db.BuildPreparation{
								BuildID: 128,
								Inputs:  map[string]db.BuildPreparationStatus{},
							}, true, nil)
						})

						Context("and creating the engine build succeeds", func() {
							var createdBuild *enginefakes.FakeBuild

							BeforeEach(func() {
								createdBuild = new(enginefakes.FakeBuild)
								fakeEngine.CreateBuildReturns(createdBuild, nil)
								fakeBuildsDB.GetBuildPreparationReturns(db.BuildPreparation{
									BuildID: 128,
									Inputs:  map[string]db.BuildPreparationStatus{},
								}, true, nil)
							})

							It("triggers a build of the job with the found inputs", func() {
								err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
								Expect(err).NotTo(HaveOccurred())

								Expect(fakePipelineDB.ScheduleBuildCallCount()).To(Equal(1))
								scheduledBuildID, jobConfig := fakePipelineDB.ScheduleBuildArgsForCall(0)
								Expect(scheduledBuildID).To(Equal(128))
								Expect(jobConfig).To(Equal(job))

								Expect(factory.CreateCallCount()).To(Equal(1))
								createJob, createResources, createInputs := factory.CreateArgsForCall(0)
								Expect(createJob).To(Equal(job))
								Expect(createResources).To(Equal(resources))
								Expect(createInputs).To(Equal(newInputs))

								Expect(fakePipelineDB.UseInputsForBuildCallCount()).To(Equal(1))
								usedBuildID, usedInputs := fakePipelineDB.UseInputsForBuildArgsForCall(0)
								Expect(usedBuildID).To(Equal(128))
								Expect(usedInputs).To(Equal(newInputs))

								Expect(fakeEngine.CreateBuildCallCount()).To(Equal(1))
								_, builtBuild, plan := fakeEngine.CreateBuildArgsForCall(0)
								Expect(builtBuild).To(Equal(db.Build{ID: 128, Name: "42"}))
								Expect(plan).To(Equal(createdPlan))
							})

							It("immediately resumes the build", func() {
								err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
								Expect(err).NotTo(HaveOccurred())

								Eventually(createdBuild.ResumeCallCount).Should(Equal(1))
							})
						})

						Context("when creating the engine build fails", func() {
							disaster := errors.New("sorry")

							BeforeEach(func() {
								factory.CreateReturns(atc.Plan{}, disaster)
							})

							It("returns no error", func() {
								err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
								Expect(err).NotTo(HaveOccurred())
							})

							It("marks the build as errored", func() {
								scheduler.BuildLatestInputs(logger, someVersions, job, resources)
								Expect(fakeBuildsDB.FinishBuildCallCount()).To(Equal(1))
								buildID, status := fakeBuildsDB.FinishBuildArgsForCall(0)
								Expect(buildID).To(Equal(128))
								Expect(status).To(Equal(db.StatusErrored))
							})
						})
					})

					Context("when the build cannot be scheduled", func() {
						BeforeEach(func() {
							fakePipelineDB.ScheduleBuildReturns(false, nil)
						})

						It("does not start a build", func() {
							err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
							Expect(err).NotTo(HaveOccurred())

							Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
						})
					})
				})

				Context("when creating the build fails", func() {
					disaster := errors.New("oh no!")

					BeforeEach(func() {
						fakePipelineDB.CreateJobBuildForCandidateInputsReturns(db.Build{}, false, disaster)
					})

					It("returns the error", func() {
						err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
						Expect(err).To(Equal(disaster))
					})

					It("does not start a build", func() {
						scheduler.BuildLatestInputs(logger, someVersions, job, resources)
						Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
					})
				})

				Context("when we do not create the build because one is already pending", func() {
					BeforeEach(func() {
						fakePipelineDB.CreateJobBuildForCandidateInputsReturns(db.Build{}, false, nil)
					})

					It("exits without error", func() {
						err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
						Expect(err).NotTo(HaveOccurred())
					})

					It("does not start a build", func() {
						scheduler.BuildLatestInputs(logger, someVersions, job, resources)
						Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
					})
				})
			})

			Context("when they are already used for a build", func() {
				BeforeEach(func() {
					fakePipelineDB.GetJobBuildForInputsReturns(db.Build{ID: 128, Name: "42"}, true, nil)
				})

				It("does not enqueue or trigger a build", func() {
					err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
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
					err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
					Expect(err).To(Equal(disaster))

					Expect(fakePipelineDB.CreateJobBuildForCandidateInputsCallCount()).To(Equal(0))
					Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
				})
			})
		})
	})

	Describe("TryNextPendingBuild", func() {
		JustBeforeEach(func() {
			scheduler.TryNextPendingBuild(logger, someVersions, job, resources).Wait()
		})

		It("does not load the versions database, as it was given one", func() {
			Expect(fakePipelineDB.LoadVersionsDBCallCount()).To(Equal(0))
		})

		Context("when a pending build is found", func() {
			pendingInputs := []db.BuildInput{
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

			pendingBuild := db.Build{
				ID:     128,
				Name:   "42",
				Status: db.StatusPending,
			}

			BeforeEach(func() {
				fakePipelineDB.GetNextPendingBuildReturns(pendingBuild, true, nil)
				fakePipelineDB.GetLatestInputVersionsReturns(pendingInputs, true, nil)
			})

			Context("when the scheduling lease cannot be acquired", func() {
				BeforeEach(func() {
					fakeBuildsDB.LeaseBuildSchedulingReturns(nil, false, nil)
				})

				It("does not schedule the build", func() {
					err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakePipelineDB.ScheduleBuildCallCount()).To(Equal(0))
				})
			})

			Context("when it can be scheduled", func() {
				BeforeEach(func() {
					fakePipelineDB.ScheduleBuildReturns(true, nil)
					fakeBuildsDB.GetBuildPreparationReturns(db.BuildPreparation{
						BuildID: pendingBuild.ID,
						Inputs:  map[string]db.BuildPreparationStatus{},
					}, true, nil)
				})

				It("should update the build preparation inputs with the correct state", func() {
					Expect(fakeBuildsDB.UpdateBuildPreparationCallCount()).To(Equal(1))
					Expect(fakeBuildsDB.UpdateBuildPreparationArgsForCall(0).Inputs).To(Equal(map[string]db.BuildPreparationStatus{
						"some-input":       db.BuildPreparationStatusNotBlocking,
						"some-other-input": db.BuildPreparationStatusNotBlocking,
					}))
				})

				Context("when creating the engine build succeeds", func() {
					var createdBuild *enginefakes.FakeBuild

					BeforeEach(func() {
						createdBuild = new(enginefakes.FakeBuild)
						fakeEngine.CreateBuildReturns(createdBuild, nil)
					})

					It("immediately resumes the build", func() {
						Eventually(createdBuild.ResumeCallCount).Should(Equal(1))
					})

					It("breaks the scheduling lease", func() {
						leasedBuildID, interval := fakeBuildsDB.LeaseBuildSchedulingArgsForCall(0)
						Expect(leasedBuildID).To(Equal(128))
						Expect(interval).To(Equal(10 * time.Second))
						Expect(lease.BreakCallCount()).To(Equal(1))
					})

					It("does not scan for new versions, and queries for the latest job inputs using the given versions dataset", func() {
						Expect(fakeScanner.ScanCallCount()).To(Equal(0))

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

						Expect(fakePipelineDB.UseInputsForBuildCallCount()).To(Equal(1))
						usedBuildID, usedInputs := fakePipelineDB.UseInputsForBuildArgsForCall(0)
						Expect(usedBuildID).To(Equal(128))
						Expect(usedInputs).To(Equal(pendingInputs))

						Expect(factory.CreateCallCount()).To(Equal(1))
						createJob, createResources, createInputs := factory.CreateArgsForCall(0)
						Expect(createJob).To(Equal(job))
						Expect(createResources).To(Equal(resources))
						Expect(createInputs).To(Equal(pendingInputs))

						Expect(fakeEngine.CreateBuildCallCount()).To(Equal(1))
						_, builtBuild, plan := fakeEngine.CreateBuildArgsForCall(0)
						Expect(builtBuild).To(Equal(pendingBuild))
						Expect(plan).To(Equal(createdPlan))
					})
				})

				Context("when creating the engine build fails", func() {
					disaster := errors.New("sorry")

					BeforeEach(func() {
						factory.CreateReturns(atc.Plan{}, disaster)
					})

					It("marks the build as errored", func() {
						Expect(fakeBuildsDB.FinishBuildCallCount()).To(Equal(1))
						buildID, status := fakeBuildsDB.FinishBuildArgsForCall(0)
						Expect(buildID).To(Equal(128))
						Expect(status).To(Equal(db.StatusErrored))
					})
				})
			})

			Context("when the build cannot be scheduled", func() {
				BeforeEach(func() {
					fakePipelineDB.ScheduleBuildReturns(false, nil)
				})

				It("does not start a build", func() {
					Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
				})

				Context("and the build's inputs are not determined", func() {
					BeforeEach(func() {
						fakePipelineDB.GetNextPendingBuildReturns(pendingBuild, true, nil)
					})

					It("does not perform any scans", func() {
						Expect(fakeScanner.ScanCallCount()).To(Equal(0))
					})
				})
			})
		})

		Context("when a pending build is not found", func() {
			BeforeEach(func() {
				fakePipelineDB.GetNextPendingBuildReturns(db.Build{}, false, nil)
			})

			It("does not start a build", func() {
				scheduler.TryNextPendingBuild(logger, someVersions, job, resources)
				Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
			})
		})

		Context("when getting the next pending build fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakePipelineDB.GetNextPendingBuildReturns(db.Build{}, false, disaster)
			})

			It("does not start a build", func() {
				scheduler.TryNextPendingBuild(logger, someVersions, job, resources)
				Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
			})
		})
	})

	Describe("TriggerImmediately", func() {
		It("creates a build without any specific inputs", func() {
			_, _, err := scheduler.TriggerImmediately(logger, job, resources)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakePipelineDB.GetLatestInputVersionsCallCount()).To(Equal(0))

			Expect(fakePipelineDB.CreateJobBuildCallCount()).To(Equal(1))

			jobName := fakePipelineDB.CreateJobBuildArgsForCall(0)
			Expect(jobName).To(Equal("some-job"))
		})

		Context("when creating the build succeeds", func() {
			createdDBBuild := db.Build{ID: 128, Name: "42"}

			pendingInputs := []db.BuildInput{
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

			BeforeEach(func() {
				fakePipelineDB.CreateJobBuildReturns(createdDBBuild, nil)
				fakePipelineDB.GetLatestInputVersionsReturns(pendingInputs, true, nil)
				fakePipelineDB.LoadVersionsDBReturns(someVersions, nil)
			})

			Context("and it can be scheduled", func() {
				BeforeEach(func() {
					fakePipelineDB.ScheduleBuildReturns(true, nil)
					fakeBuildsDB.GetBuildPreparationReturns(db.BuildPreparation{
						BuildID: createdDBBuild.ID,
						Inputs:  map[string]db.BuildPreparationStatus{},
					}, true, nil)
				})

				Context("and creating the engine build succeeds", func() {
					var createdBuild *enginefakes.FakeBuild

					BeforeEach(func() {
						createdBuild = new(enginefakes.FakeBuild)
						fakeEngine.CreateBuildReturns(createdBuild, nil)
					})

					Context("something about how were checking resouces and doing things", func() { //TODO CHANGE DIS
						It("correctly updates the build prep for every input being used", func() {
							_, wg, err := scheduler.TriggerImmediately(logger, job, resources)
							Expect(err).ToNot(HaveOccurred())
							wg.Wait()

							Expect(fakeBuildsDB.UpdateBuildPreparationCallCount()).To(Equal(3))

							Expect(fakeBuildsDB.UpdateBuildPreparationArgsForCall(0).Inputs).To(Equal(map[string]db.BuildPreparationStatus{
								"some-input":       db.BuildPreparationStatusBlocking,
								"some-other-input": db.BuildPreparationStatusBlocking,
							}))

							Expect(fakeBuildsDB.UpdateBuildPreparationArgsForCall(2).Inputs).To(Equal(map[string]db.BuildPreparationStatus{
								"some-input":       db.BuildPreparationStatusNotBlocking,
								"some-other-input": db.BuildPreparationStatusNotBlocking,
							}))
						})
					})

					It("scans for new versions for each input, and queries for the latest job inputs", func() {
						_, w, err := scheduler.TriggerImmediately(logger, job, resources)
						Expect(err).NotTo(HaveOccurred())

						w.Wait()

						Expect(fakeScanner.ScanCallCount()).To(Equal(2))

						_, resourceName := fakeScanner.ScanArgsForCall(0)
						Expect(resourceName).To(Equal("some-resource"))

						_, resourceName = fakeScanner.ScanArgsForCall(1)
						Expect(resourceName).To(Equal("some-other-resource"))

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

						Expect(fakePipelineDB.UseInputsForBuildCallCount()).To(Equal(1))
						usedBuildID, usedInputs := fakePipelineDB.UseInputsForBuildArgsForCall(0)
						Expect(usedBuildID).To(Equal(128))
						Expect(usedInputs).To(Equal(pendingInputs))

						Expect(factory.CreateCallCount()).To(Equal(1))
						createJob, createResources, createInputs := factory.CreateArgsForCall(0)
						Expect(createJob).To(Equal(job))
						Expect(createResources).To(Equal(resources))
						Expect(createInputs).To(Equal(pendingInputs))

						Expect(fakeEngine.CreateBuildCallCount()).To(Equal(1))
						_, builtBuild, plan := fakeEngine.CreateBuildArgsForCall(0)
						Expect(builtBuild).To(Equal(createdDBBuild))
						Expect(plan).To(Equal(createdPlan))
					})

					Context("when scanning fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeScanner.ScanReturns(disaster)
						})

						It("errors the build", func() {
							_, w, err := scheduler.TriggerImmediately(logger, job, resources)
							Expect(err).NotTo(HaveOccurred())

							w.Wait()

							Expect(fakeBuildsDB.ErrorBuildCallCount()).To(Equal(1))

							buildID, err := fakeBuildsDB.ErrorBuildArgsForCall(0)
							Expect(buildID).To(Equal(128))
							Expect(err).To(Equal(disaster))
						})
					})

					Context("when loading the versions dataset fails", func() {
						BeforeEach(func() {
							fakePipelineDB.LoadVersionsDBReturns(nil, errors.New("oh no!"))
						})

						It("does not run the build", func() {
							_, w, err := scheduler.TriggerImmediately(logger, job, resources)
							Expect(err).NotTo(HaveOccurred())

							w.Wait()

							Expect(fakePipelineDB.GetLatestInputVersionsCallCount()).To(Equal(0))
						})

						It("does not error the build, as it may have been an ephemeral database issue", func() {
							_, w, err := scheduler.TriggerImmediately(logger, job, resources)
							Expect(err).NotTo(HaveOccurred())

							w.Wait()

							Expect(fakeBuildsDB.ErrorBuildCallCount()).To(Equal(0))
						})
					})

					It("triggers a build of the job with the found inputs", func() {
						build, w, err := scheduler.TriggerImmediately(logger, job, resources)
						Expect(err).NotTo(HaveOccurred())
						Expect(build).To(Equal(db.Build{ID: 128, Name: "42"}))

						w.Wait()

						Expect(fakePipelineDB.ScheduleBuildCallCount()).To(Equal(1))
						scheduledBuildID, jobConfig := fakePipelineDB.ScheduleBuildArgsForCall(0)
						Expect(scheduledBuildID).To(Equal(128))
						Expect(jobConfig).To(Equal(job))

						Expect(fakePipelineDB.UseInputsForBuildCallCount()).To(Equal(1))
						usedBuildID, usedInputs := fakePipelineDB.UseInputsForBuildArgsForCall(0)
						Expect(usedBuildID).To(Equal(128))
						Expect(usedInputs).To(Equal(pendingInputs))

						Expect(factory.CreateCallCount()).To(Equal(1))
						createJob, createResources, createInputs := factory.CreateArgsForCall(0)
						Expect(createJob).To(Equal(job))
						Expect(createResources).To(Equal(resources))
						Expect(createInputs).To(Equal(pendingInputs))

						Expect(fakeEngine.CreateBuildCallCount()).To(Equal(1))
						_, builtBuild, plan := fakeEngine.CreateBuildArgsForCall(0)
						Expect(builtBuild).To(Equal(db.Build{ID: 128, Name: "42"}))
						Expect(plan).To(Equal(createdPlan))
					})

					It("immediately resumes the build", func() {
						build, w, err := scheduler.TriggerImmediately(logger, job, resources)
						Expect(err).NotTo(HaveOccurred())
						Expect(build).To(Equal(db.Build{ID: 128, Name: "42"}))

						w.Wait()

						Eventually(createdBuild.ResumeCallCount).Should(Equal(1))
					})
				})

				Context("when creating the engine build fails", func() {
					disaster := errors.New("sorry")

					BeforeEach(func() {
						factory.CreateReturns(atc.Plan{}, disaster)
					})

					It("returns no error", func() {
						_, _, err := scheduler.TriggerImmediately(logger, job, resources)
						Expect(err).NotTo(HaveOccurred())
					})

					It("marks the build as errored", func() {
						_, w, _ := scheduler.TriggerImmediately(logger, job, resources)
						w.Wait()
						Expect(fakeBuildsDB.FinishBuildCallCount()).To(Equal(1))
						buildID, status := fakeBuildsDB.FinishBuildArgsForCall(0)
						Expect(buildID).To(Equal(128))
						Expect(status).To(Equal(db.StatusErrored))
					})
				})
			})

			Context("when the build cannot be scheduled", func() {
				BeforeEach(func() {
					fakePipelineDB.ScheduleBuildReturns(false, nil)
				})

				It("does not start a build", func() {
					_, _, err := scheduler.TriggerImmediately(logger, job, resources)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
				})
			})
		})

		Context("when creating the build fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakePipelineDB.CreateJobBuildReturns(db.Build{}, disaster)
			})

			It("returns the error", func() {
				_, _, err := scheduler.TriggerImmediately(logger, job, resources)
				Expect(err).To(Equal(disaster))
			})

			It("does not start a build", func() {
				scheduler.TriggerImmediately(logger, job, resources)
				Expect(fakeEngine.CreateBuildCallCount()).To(Equal(0))
			})
		})
	})
})
