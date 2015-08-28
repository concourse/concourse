package scheduler_test

import (
	"errors"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
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

		createdPlan atc.Plan

		job       atc.JobConfig
		resources atc.ResourceConfigs

		scheduler *Scheduler

		someVersions algorithm.VersionsDB

		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakePipelineDB = new(fakes.FakePipelineDB)
		fakeBuildsDB = new(fakes.FakeBuildsDB)
		factory = new(fakes.FakeBuildFactory)
		fakeEngine = new(enginefakes.FakeEngine)
		fakeScanner = new(fakes.FakeScanner)

		someVersions = []algorithm.BuildOutput{
			{VersionID: 1, ResourceID: 2, BuildID: 3, JobID: 4},
			{VersionID: 5, ResourceID: 6, BuildID: 7, JobID: 8},
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

			InputConfigs: []atc.JobInputConfig{
				{
					RawName:  "some-input",
					Resource: "some-resource",
					Params:   atc.Params{"some": "params"},
					Trigger:  true,
				},
				{
					RawName:  "some-other-input",
					Resource: "some-other-resource",
					Params:   atc.Params{"some": "params"},
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
	})

	Describe("BuildLatestInputs", func() {
		Context("when no inputs are available", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakePipelineDB.GetLatestInputVersionsReturns(nil, disaster)
			})

			It("returns the error", func() {
				err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
				Ω(err).Should(Equal(disaster))
			})

			It("does not trigger a build", func() {
				scheduler.BuildLatestInputs(logger, someVersions, job, resources)
				Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
			})
		})

		Context("when the job has no inputs", func() {
			BeforeEach(func() {
				job.InputConfigs = []atc.JobInputConfig{}
			})

			It("succeeds", func() {
				err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("does not load the versions database, as it was given one", func() {
				scheduler.BuildLatestInputs(logger, someVersions, job, resources)

				Ω(fakePipelineDB.LoadVersionsDBCallCount()).Should(Equal(0))
			})

			It("does not try to fetch inputs from the database", func() {
				scheduler.BuildLatestInputs(logger, someVersions, job, resources)

				Ω(fakePipelineDB.GetLatestInputVersionsCallCount()).Should(BeZero())
			})

			It("does not trigger a build", func() {
				scheduler.BuildLatestInputs(logger, someVersions, job, resources)

				Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
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
				fakePipelineDB.GetLatestInputVersionsReturns(newInputs, nil)
			})

			It("does not load the versions database, as it was given one", func() {
				scheduler.BuildLatestInputs(logger, someVersions, job, resources)

				Ω(fakePipelineDB.LoadVersionsDBCallCount()).Should(Equal(0))
			})

			It("checks if they are already used for a build", func() {
				err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakePipelineDB.GetLatestInputVersionsCallCount()).Should(Equal(1))
				versions, jobName, inputs := fakePipelineDB.GetLatestInputVersionsArgsForCall(0)
				Ω(versions).Should(Equal(someVersions))
				Ω(jobName).Should(Equal(job.Name))
				Ω(inputs).Should(Equal([]atc.JobInput{
					{
						Name:     "some-input",
						Resource: "some-resource",
						Trigger:  true,
					},
					{
						Name:     "some-other-input",
						Resource: "some-other-resource",
						Trigger:  true,
					},
				}))

				Ω(fakePipelineDB.GetJobBuildForInputsCallCount()).Should(Equal(1))

				checkedJob, checkedInputs := fakePipelineDB.GetJobBuildForInputsArgsForCall(0)
				Ω(checkedJob).Should(Equal("some-job"))
				Ω(checkedInputs).Should(ConsistOf(newInputs))
			})

			Context("and the job has inputs configured to not trigger when they change", func() {
				BeforeEach(func() {
					job.InputConfigs = append(job.InputConfigs, atc.JobInputConfig{
						Resource: "some-non-triggering-resource",
						Trigger:  false,
					})

					foundInputsWithCheck := append(
						newInputs,
						db.BuildInput{
							Name: "some-non-triggering-resource",
							VersionedResource: db.VersionedResource{
								Resource: "some-non-triggering-resource",
								Version:  db.Version{"version": 3},
							},
						},
					)

					fakePipelineDB.GetLatestInputVersionsReturns(foundInputsWithCheck, nil)
				})

				It("excludes them from the inputs when checking for a build", func() {
					err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakePipelineDB.GetJobBuildForInputsCallCount()).Should(Equal(1))

					checkedJob, checkedInputs := fakePipelineDB.GetJobBuildForInputsArgsForCall(0)
					Ω(checkedJob).Should(Equal("some-job"))
					Ω(checkedInputs).Should(Equal(newInputs))
				})
			})

			Context("and all inputs are configured not to trigger", func() {
				BeforeEach(func() {
					for i, input := range job.InputConfigs {
						noChecking := input
						noChecking.Trigger = false

						job.InputConfigs[i] = noChecking
					}
				})

				It("does not check for builds for the inputs", func() {
					err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakePipelineDB.GetJobBuildForInputsCallCount()).Should(Equal(0))
				})

				It("does not create a build", func() {
					err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakePipelineDB.CreateJobBuildForCandidateInputsCallCount()).Should(Equal(0))
				})

				It("does not trigger a build", func() {
					err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
				})
			})

			Context("and they are not used for a build", func() {
				BeforeEach(func() {
					fakePipelineDB.GetJobBuildForInputsReturns(db.Build{}, errors.New("no build"))
				})

				It("creates a build with the found inputs", func() {
					err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakePipelineDB.CreateJobBuildForCandidateInputsCallCount()).Should(Equal(1))
					buildJob := fakePipelineDB.CreateJobBuildForCandidateInputsArgsForCall(0)
					Ω(buildJob).Should(Equal("some-job"))
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
							nil,
						)
					})

					Context("and it can be scheduled", func() {
						BeforeEach(func() {
							fakePipelineDB.ScheduleBuildReturns(true, nil)
						})

						Context("and creating the engine build succeeds", func() {
							var createdBuild *enginefakes.FakeBuild

							BeforeEach(func() {
								createdBuild = new(enginefakes.FakeBuild)
								fakeEngine.CreateBuildReturns(createdBuild, nil)
							})

							It("triggers a build of the job with the found inputs", func() {
								err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
								Ω(err).ShouldNot(HaveOccurred())

								Ω(fakePipelineDB.ScheduleBuildCallCount()).Should(Equal(1))
								scheduledBuildID, jobConfig := fakePipelineDB.ScheduleBuildArgsForCall(0)
								Ω(scheduledBuildID).Should(Equal(128))
								Ω(jobConfig).Should(Equal(job))

								Ω(factory.CreateCallCount()).Should(Equal(1))
								createJob, createResources, createInputs := factory.CreateArgsForCall(0)
								Ω(createJob).Should(Equal(job))
								Ω(createResources).Should(Equal(resources))
								Ω(createInputs).Should(Equal(newInputs))

								Ω(fakePipelineDB.UseInputsForBuildCallCount()).Should(Equal(1))
								usedBuildID, usedInputs := fakePipelineDB.UseInputsForBuildArgsForCall(0)
								Ω(usedBuildID).Should(Equal(128))
								Ω(usedInputs).Should(Equal(newInputs))

								Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(1))
								builtBuild, plan := fakeEngine.CreateBuildArgsForCall(0)
								Ω(builtBuild).Should(Equal(db.Build{ID: 128, Name: "42"}))
								Ω(plan).Should(Equal(createdPlan))
							})

							It("immediately resumes the build", func() {
								err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
								Ω(err).ShouldNot(HaveOccurred())

								Eventually(createdBuild.ResumeCallCount).Should(Equal(1))
							})
						})
					})

					Context("when the build cannot be scheduled", func() {
						BeforeEach(func() {
							fakePipelineDB.ScheduleBuildReturns(false, nil)
						})

						It("does not start a build", func() {
							err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
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
						Ω(err).Should(Equal(disaster))
					})

					It("does not start a build", func() {
						scheduler.BuildLatestInputs(logger, someVersions, job, resources)
						Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
					})
				})

				Context("when we do not create the build because one is already pending", func() {
					BeforeEach(func() {
						fakePipelineDB.CreateJobBuildForCandidateInputsReturns(db.Build{}, false, nil)
					})

					It("exits without error", func() {
						err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
						Ω(err).ShouldNot(HaveOccurred())
					})

					It("does not start a build", func() {
						scheduler.BuildLatestInputs(logger, someVersions, job, resources)
						Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
					})
				})
			})

			Context("but they are already used for a build", func() {
				BeforeEach(func() {
					fakePipelineDB.GetJobBuildForInputsReturns(db.Build{ID: 128, Name: "42"}, nil)
				})

				It("does not trigger a build", func() {
					err := scheduler.BuildLatestInputs(logger, someVersions, job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
				})
			})
		})
	})

	Describe("TryNextPendingBuild", func() {
		JustBeforeEach(func() {
			scheduler.TryNextPendingBuild(logger, someVersions, job, resources).Wait()
		})

		It("does not load the versions database, as it was given one", func() {
			Ω(fakePipelineDB.LoadVersionsDBCallCount()).Should(Equal(0))
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
				fakePipelineDB.GetNextPendingBuildReturns(pendingBuild, nil)

				fakePipelineDB.GetLatestInputVersionsReturns(pendingInputs, nil)
			})

			Context("and it can be scheduled", func() {
				BeforeEach(func() {
					fakePipelineDB.ScheduleBuildReturns(true, nil)
				})

				Context("and creating the engine build succeeds", func() {
					var createdBuild *enginefakes.FakeBuild

					BeforeEach(func() {
						createdBuild = new(enginefakes.FakeBuild)
						fakeEngine.CreateBuildReturns(createdBuild, nil)
					})

					It("immediately resumes the build", func() {
						Eventually(createdBuild.ResumeCallCount).Should(Equal(1))
					})

					It("does not scan for new versions, and queries for the latest job inputs using the given versions dataset", func() {
						Ω(fakeScanner.ScanCallCount()).Should(Equal(0))

						Ω(fakePipelineDB.GetLatestInputVersionsCallCount()).Should(Equal(1))
						versions, jobName, inputConfigs := fakePipelineDB.GetLatestInputVersionsArgsForCall(0)
						Ω(versions).Should(Equal(someVersions))
						Ω(jobName).Should(Equal(job.Name))
						Ω(inputConfigs).Should(Equal([]atc.JobInput{
							{
								Name:     "some-input",
								Resource: "some-resource",
								Trigger:  true,
							},
							{
								Name:     "some-other-input",
								Resource: "some-other-resource",
								Trigger:  true,
							},
						}))

						Ω(fakePipelineDB.UseInputsForBuildCallCount()).Should(Equal(1))
						usedBuildID, usedInputs := fakePipelineDB.UseInputsForBuildArgsForCall(0)
						Ω(usedBuildID).Should(Equal(128))
						Ω(usedInputs).Should(Equal(pendingInputs))

						Ω(factory.CreateCallCount()).Should(Equal(1))
						createJob, createResources, createInputs := factory.CreateArgsForCall(0)
						Ω(createJob).Should(Equal(job))
						Ω(createResources).Should(Equal(resources))
						Ω(createInputs).Should(Equal(pendingInputs))

						Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(1))
						builtBuild, plan := fakeEngine.CreateBuildArgsForCall(0)
						Ω(builtBuild).Should(Equal(pendingBuild))
						Ω(plan).Should(Equal(createdPlan))
					})
				})
			})

			Context("when the build cannot be scheduled", func() {
				BeforeEach(func() {
					fakePipelineDB.ScheduleBuildReturns(false, nil)
				})

				It("does not start a build", func() {
					Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
				})

				Context("and the build's inputs are not determined", func() {
					BeforeEach(func() {
						fakePipelineDB.GetNextPendingBuildReturns(pendingBuild, nil)
					})

					It("does not perform any scans", func() {
						Ω(fakeScanner.ScanCallCount()).Should(Equal(0))
					})
				})
			})
		})

		Context("when a pending build is not found", func() {
			BeforeEach(func() {
				fakePipelineDB.GetNextPendingBuildReturns(db.Build{}, db.ErrNoBuild)
			})

			It("does not start a build", func() {
				scheduler.TryNextPendingBuild(logger, someVersions, job, resources)
				Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
			})
		})

		Context("when getting the next pending build fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakePipelineDB.GetNextPendingBuildReturns(db.Build{}, disaster)
			})

			It("does not start a build", func() {
				scheduler.TryNextPendingBuild(logger, someVersions, job, resources)
				Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
			})
		})
	})

	Describe("TriggerImmediately", func() {
		It("creates a build without any specific inputs", func() {
			_, _, err := scheduler.TriggerImmediately(logger, job, resources)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakePipelineDB.GetLatestInputVersionsCallCount()).Should(Equal(0))

			Ω(fakePipelineDB.CreateJobBuildCallCount()).Should(Equal(1))

			jobName := fakePipelineDB.CreateJobBuildArgsForCall(0)
			Ω(jobName).Should(Equal("some-job"))
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
				fakePipelineDB.GetLatestInputVersionsReturns(pendingInputs, nil)
				fakePipelineDB.LoadVersionsDBReturns(someVersions, nil)
			})

			Context("and it can be scheduled", func() {
				BeforeEach(func() {
					fakePipelineDB.ScheduleBuildReturns(true, nil)
				})

				Context("and creating the engine build succeeds", func() {
					var createdBuild *enginefakes.FakeBuild

					BeforeEach(func() {
						createdBuild = new(enginefakes.FakeBuild)
						fakeEngine.CreateBuildReturns(createdBuild, nil)
					})

					It("scans for new versions for each input, and queries for the latest job inputs", func() {
						_, w, err := scheduler.TriggerImmediately(logger, job, resources)
						Ω(err).ShouldNot(HaveOccurred())

						w.Wait()

						Ω(fakeScanner.ScanCallCount()).Should(Equal(2))

						_, resourceName := fakeScanner.ScanArgsForCall(0)
						Ω(resourceName).Should(Equal("some-resource"))

						_, resourceName = fakeScanner.ScanArgsForCall(1)
						Ω(resourceName).Should(Equal("some-other-resource"))

						Ω(fakePipelineDB.GetLatestInputVersionsCallCount()).Should(Equal(1))
						versions, jobName, inputConfigs := fakePipelineDB.GetLatestInputVersionsArgsForCall(0)
						Ω(versions).Should(Equal(someVersions))
						Ω(jobName).Should(Equal(job.Name))
						Ω(inputConfigs).Should(Equal([]atc.JobInput{
							{
								Name:     "some-input",
								Resource: "some-resource",
								Trigger:  true,
							},
							{
								Name:     "some-other-input",
								Resource: "some-other-resource",
								Trigger:  true,
							},
						}))

						Ω(fakePipelineDB.UseInputsForBuildCallCount()).Should(Equal(1))
						usedBuildID, usedInputs := fakePipelineDB.UseInputsForBuildArgsForCall(0)
						Ω(usedBuildID).Should(Equal(128))
						Ω(usedInputs).Should(Equal(pendingInputs))

						Ω(factory.CreateCallCount()).Should(Equal(1))
						createJob, createResources, createInputs := factory.CreateArgsForCall(0)
						Ω(createJob).Should(Equal(job))
						Ω(createResources).Should(Equal(resources))
						Ω(createInputs).Should(Equal(pendingInputs))

						Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(1))
						builtBuild, plan := fakeEngine.CreateBuildArgsForCall(0)
						Ω(builtBuild).Should(Equal(createdDBBuild))
						Ω(plan).Should(Equal(createdPlan))
					})

					Context("when scanning fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeScanner.ScanReturns(disaster)
						})

						It("errors the build", func() {
							_, w, err := scheduler.TriggerImmediately(logger, job, resources)
							Ω(err).ShouldNot(HaveOccurred())

							w.Wait()

							Ω(fakeBuildsDB.ErrorBuildCallCount()).Should(Equal(1))

							buildID, err := fakeBuildsDB.ErrorBuildArgsForCall(0)
							Ω(buildID).Should(Equal(128))
							Ω(err).Should(Equal(disaster))
						})
					})

					Context("when loading the versions dataset fails", func() {
						BeforeEach(func() {
							fakePipelineDB.LoadVersionsDBReturns(nil, errors.New("oh no!"))
						})

						It("does not run the build", func() {
							_, w, err := scheduler.TriggerImmediately(logger, job, resources)
							Ω(err).ShouldNot(HaveOccurred())

							w.Wait()

							Ω(fakePipelineDB.GetLatestInputVersionsCallCount()).Should(Equal(0))
						})

						It("does not error the build, as it may have been an ephemeral database issue", func() {
							_, w, err := scheduler.TriggerImmediately(logger, job, resources)
							Ω(err).ShouldNot(HaveOccurred())

							w.Wait()

							Ω(fakeBuildsDB.ErrorBuildCallCount()).Should(Equal(0))
						})
					})

					It("triggers a build of the job with the found inputs", func() {
						build, w, err := scheduler.TriggerImmediately(logger, job, resources)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(build).Should(Equal(db.Build{ID: 128, Name: "42"}))

						w.Wait()

						Ω(fakePipelineDB.ScheduleBuildCallCount()).Should(Equal(1))
						scheduledBuildID, jobConfig := fakePipelineDB.ScheduleBuildArgsForCall(0)
						Ω(scheduledBuildID).Should(Equal(128))
						Ω(jobConfig).Should(Equal(job))

						Ω(fakePipelineDB.UseInputsForBuildCallCount()).Should(Equal(1))
						usedBuildID, usedInputs := fakePipelineDB.UseInputsForBuildArgsForCall(0)
						Ω(usedBuildID).Should(Equal(128))
						Ω(usedInputs).Should(Equal(pendingInputs))

						Ω(factory.CreateCallCount()).Should(Equal(1))
						createJob, createResources, createInputs := factory.CreateArgsForCall(0)
						Ω(createJob).Should(Equal(job))
						Ω(createResources).Should(Equal(resources))
						Ω(createInputs).Should(Equal(pendingInputs))

						Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(1))
						builtBuild, plan := fakeEngine.CreateBuildArgsForCall(0)
						Ω(builtBuild).Should(Equal(db.Build{ID: 128, Name: "42"}))
						Ω(plan).Should(Equal(createdPlan))
					})

					It("immediately resumes the build", func() {
						build, w, err := scheduler.TriggerImmediately(logger, job, resources)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(build).Should(Equal(db.Build{ID: 128, Name: "42"}))

						w.Wait()

						Eventually(createdBuild.ResumeCallCount).Should(Equal(1))
					})
				})
			})

			Context("when the build cannot be scheduled", func() {
				BeforeEach(func() {
					fakePipelineDB.ScheduleBuildReturns(false, nil)
				})

				It("does not start a build", func() {
					_, _, err := scheduler.TriggerImmediately(logger, job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
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
				Ω(err).Should(Equal(disaster))
			})

			It("does not start a build", func() {
				scheduler.TriggerImmediately(logger, job, resources)
				Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
			})
		})
	})
})
