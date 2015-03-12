package scheduler_test

import (
	"errors"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	enginefakes "github.com/concourse/atc/engine/fakes"
	. "github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/fakes"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scheduler", func() {
	var (
		fakeSchedulerDB *fakes.FakeSchedulerDB
		factory         *fakes.FakeBuildFactory
		fakeEngine      *enginefakes.FakeEngine
		fakeScanner     *fakes.FakeScanner

		createdPlan atc.Plan

		job       atc.JobConfig
		resources atc.ResourceConfigs

		scheduler *Scheduler

		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeSchedulerDB = new(fakes.FakeSchedulerDB)
		factory = new(fakes.FakeBuildFactory)
		fakeEngine = new(enginefakes.FakeEngine)
		fakeScanner = new(fakes.FakeScanner)

		createdPlan = atc.Plan{
			Task: &atc.TaskPlan{
				Config: &atc.BuildConfig{
					Run: atc.BuildRunConfig{Path: "some-build"},
				},
			},
		}

		factory.CreateReturns(createdPlan, nil)

		scheduler = &Scheduler{
			DB:      fakeSchedulerDB,
			Factory: factory,
			Engine:  fakeEngine,
			Scanner: fakeScanner,
		}

		logger = lagertest.NewTestLogger("test")

		yes := true
		job = atc.JobConfig{
			Name: "some-job",

			Serial: true,

			InputConfigs: []atc.JobInputConfig{
				{
					RawName:    "some-input",
					Resource:   "some-resource",
					Params:     atc.Params{"some": "params"},
					RawTrigger: &yes,
				},
				{
					RawName:    "some-other-input",
					Resource:   "some-other-resource",
					Params:     atc.Params{"some": "params"},
					RawTrigger: &yes,
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

	Describe("TrackInFlightBuilds", func() {
		var (
			inFlightBuilds []db.Build

			engineBuilds []*enginefakes.FakeBuild
		)

		BeforeEach(func() {
			inFlightBuilds = []db.Build{
				{ID: 1},
				{ID: 2},
				{ID: 3},
			}

			engineBuilds = []*enginefakes.FakeBuild{
				new(enginefakes.FakeBuild),
				new(enginefakes.FakeBuild),
				new(enginefakes.FakeBuild),
			}

			fakeSchedulerDB.GetAllStartedBuildsReturns(inFlightBuilds, nil)

			fakeEngine.LookupBuildStub = func(build db.Build) (engine.Build, error) {
				return engineBuilds[build.ID-1], nil
			}
		})

		It("resumes all currently in-flight builds", func() {
			err := scheduler.TrackInFlightBuilds(logger)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(engineBuilds[0].ResumeCallCount).Should(Equal(1))
			Eventually(engineBuilds[1].ResumeCallCount).Should(Equal(1))
			Eventually(engineBuilds[2].ResumeCallCount).Should(Equal(1))
		})

		Context("when a build cannot be looked up", func() {
			BeforeEach(func() {
				fakeEngine.LookupBuildReturns(nil, errors.New("nope"))
			})

			It("saves its status as errored", func() {
				err := scheduler.TrackInFlightBuilds(logger)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeSchedulerDB.FinishBuildCallCount()).Should(Equal(3))

				savedBuilID1, status1 := fakeSchedulerDB.FinishBuildArgsForCall(0)
				Ω(savedBuilID1).Should(Equal(1))
				Ω(status1).Should(Equal(db.StatusErrored))

				savedBuilID2, status2 := fakeSchedulerDB.FinishBuildArgsForCall(1)
				Ω(savedBuilID2).Should(Equal(2))
				Ω(status2).Should(Equal(db.StatusErrored))

				savedBuilID3, status3 := fakeSchedulerDB.FinishBuildArgsForCall(2)
				Ω(savedBuilID3).Should(Equal(3))
				Ω(status3).Should(Equal(db.StatusErrored))
			})
		})
	})

	Describe("BuildLatestInputs", func() {
		Context("when no inputs are available", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeSchedulerDB.GetLatestInputVersionsReturns(nil, disaster)
			})

			It("returns the error", func() {
				err := scheduler.BuildLatestInputs(logger, job, resources)
				Ω(err).Should(Equal(disaster))
			})

			It("does not trigger a build", func() {
				scheduler.BuildLatestInputs(logger, job, resources)

				Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
			})
		})

		Context("when the job has no inputs", func() {
			BeforeEach(func() {
				job.InputConfigs = []atc.JobInputConfig{}
			})

			It("succeeds", func() {
				err := scheduler.BuildLatestInputs(logger, job, resources)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("does not try to fetch inputs from the database", func() {
				scheduler.BuildLatestInputs(logger, job, resources)

				Ω(fakeSchedulerDB.GetLatestInputVersionsCallCount()).Should(BeZero())
			})

			It("does not trigger a build", func() {
				scheduler.BuildLatestInputs(logger, job, resources)

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
				fakeSchedulerDB.GetLatestInputVersionsReturns(newInputs, nil)
			})

			It("checks if they are already used for a build", func() {
				err := scheduler.BuildLatestInputs(logger, job, resources)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeSchedulerDB.GetLatestInputVersionsCallCount()).Should(Equal(1))
				Ω(fakeSchedulerDB.GetLatestInputVersionsArgsForCall(0)).Should(Equal([]atc.JobInput{
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

				Ω(fakeSchedulerDB.GetJobBuildForInputsCallCount()).Should(Equal(1))

				checkedJob, checkedInputs := fakeSchedulerDB.GetJobBuildForInputsArgsForCall(0)
				Ω(checkedJob).Should(Equal("some-job"))
				Ω(checkedInputs).Should(ConsistOf(newInputs))
			})

			Context("and the job has inputs configured to not trigger when they change", func() {
				BeforeEach(func() {
					trigger := false

					job.InputConfigs = append(job.InputConfigs, atc.JobInputConfig{
						Resource:   "some-non-triggering-resource",
						RawTrigger: &trigger,
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

					fakeSchedulerDB.GetLatestInputVersionsReturns(foundInputsWithCheck, nil)
				})

				It("excludes them from the inputs when checking for a build", func() {
					err := scheduler.BuildLatestInputs(logger, job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakeSchedulerDB.GetJobBuildForInputsCallCount()).Should(Equal(1))

					checkedJob, checkedInputs := fakeSchedulerDB.GetJobBuildForInputsArgsForCall(0)
					Ω(checkedJob).Should(Equal("some-job"))
					Ω(checkedInputs).Should(Equal(newInputs))
				})
			})

			Context("and all inputs are configured not to trigger", func() {
				BeforeEach(func() {
					trigger := false

					for i, input := range job.InputConfigs {
						noChecking := input
						noChecking.RawTrigger = &trigger

						job.InputConfigs[i] = noChecking
					}
				})

				It("does not check for builds for the inputs", func() {
					err := scheduler.BuildLatestInputs(logger, job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakeSchedulerDB.GetJobBuildForInputsCallCount()).Should(Equal(0))
				})

				It("does not create a build", func() {
					err := scheduler.BuildLatestInputs(logger, job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakeSchedulerDB.CreateJobBuildWithInputsCallCount()).Should(Equal(0))
				})

				It("does not trigger a build", func() {
					err := scheduler.BuildLatestInputs(logger, job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
				})
			})

			Context("and they are not used for a build", func() {
				BeforeEach(func() {
					fakeSchedulerDB.GetJobBuildForInputsReturns(db.Build{}, errors.New("no build"))
				})

				It("creates a build with the found inputs", func() {
					err := scheduler.BuildLatestInputs(logger, job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakeSchedulerDB.CreateJobBuildWithInputsCallCount()).Should(Equal(1))
					buildJob, buildInputs := fakeSchedulerDB.CreateJobBuildWithInputsArgsForCall(0)
					Ω(buildJob).Should(Equal("some-job"))
					Ω(buildInputs).Should(Equal(newInputs))
				})

				Context("when creating the build succeeds", func() {
					BeforeEach(func() {
						fakeSchedulerDB.CreateJobBuildWithInputsReturns(db.Build{ID: 128, Name: "42"}, nil)
					})

					Context("and it can be scheduled", func() {
						BeforeEach(func() {
							fakeSchedulerDB.ScheduleBuildReturns(true, nil)
						})

						Context("and creating the engine build succeeds", func() {
							var createdBuild *enginefakes.FakeBuild

							BeforeEach(func() {
								createdBuild = new(enginefakes.FakeBuild)
								fakeEngine.CreateBuildReturns(createdBuild, nil)
							})

							It("triggers a build of the job with the found inputs", func() {
								err := scheduler.BuildLatestInputs(logger, job, resources)
								Ω(err).ShouldNot(HaveOccurred())

								Ω(fakeSchedulerDB.ScheduleBuildCallCount()).Should(Equal(1))
								scheduledBuildID, serial := fakeSchedulerDB.ScheduleBuildArgsForCall(0)
								Ω(scheduledBuildID).Should(Equal(128))
								Ω(serial).Should(Equal(job.Serial))

								Ω(factory.CreateCallCount()).Should(Equal(1))
								createJob, createResources, createInputs := factory.CreateArgsForCall(0)
								Ω(createJob).Should(Equal(job))
								Ω(createResources).Should(Equal(resources))
								Ω(createInputs).Should(Equal(newInputs))

								Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(1))
								builtBuild, plan := fakeEngine.CreateBuildArgsForCall(0)
								Ω(builtBuild).Should(Equal(db.Build{ID: 128, Name: "42"}))
								Ω(plan).Should(Equal(createdPlan))
							})

							It("immediately resumes the build", func() {
								err := scheduler.BuildLatestInputs(logger, job, resources)
								Ω(err).ShouldNot(HaveOccurred())

								Eventually(createdBuild.ResumeCallCount).Should(Equal(1))
							})
						})
					})

					Context("when the build cannot be scheduled", func() {
						BeforeEach(func() {
							fakeSchedulerDB.ScheduleBuildReturns(false, nil)
						})

						It("does not start a build", func() {
							err := scheduler.BuildLatestInputs(logger, job, resources)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
						})
					})
				})

				Context("when creating the build fails", func() {
					disaster := errors.New("oh no!")

					BeforeEach(func() {
						fakeSchedulerDB.CreateJobBuildWithInputsReturns(db.Build{}, disaster)
					})

					It("returns the error", func() {
						err := scheduler.BuildLatestInputs(logger, job, resources)
						Ω(err).Should(Equal(disaster))
					})

					It("does not start a build", func() {
						scheduler.BuildLatestInputs(logger, job, resources)
						Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
					})
				})
			})

			Context("but they are already used for a build", func() {
				BeforeEach(func() {
					fakeSchedulerDB.GetJobBuildForInputsReturns(db.Build{ID: 128, Name: "42"}, nil)
				})

				It("does not trigger a build", func() {
					err := scheduler.BuildLatestInputs(logger, job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
				})
			})
		})
	})

	Describe("TryNextPendingBuild", func() {
		JustBeforeEach(func() {
			scheduler.TryNextPendingBuild(logger, job, resources).Wait()
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
				fakeSchedulerDB.GetNextPendingBuildReturns(pendingBuild, pendingInputs, nil)
			})

			Context("and it can be scheduled", func() {
				BeforeEach(func() {
					fakeSchedulerDB.ScheduleBuildReturns(true, nil)
				})

				Context("and creating the engine build succeeds", func() {
					var createdBuild *enginefakes.FakeBuild

					BeforeEach(func() {
						createdBuild = new(enginefakes.FakeBuild)
						fakeEngine.CreateBuildReturns(createdBuild, nil)
					})

					It("builds it", func() {
						Ω(fakeSchedulerDB.ScheduleBuildCallCount()).Should(Equal(1))
						scheduledBuildID, serial := fakeSchedulerDB.ScheduleBuildArgsForCall(0)
						Ω(scheduledBuildID).Should(Equal(128))
						Ω(serial).Should(Equal(job.Serial))

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

					It("immediately resumes the build", func() {
						Eventually(createdBuild.ResumeCallCount).Should(Equal(1))
					})

					Context("when the inputs are only partially determined", func() {
						// this can happen if the job config changes after a pending build is created

						BeforeEach(func() {
							fakeSchedulerDB.GetNextPendingBuildReturns(pendingBuild, pendingInputs[1:], nil)
						})

						It("marks the build as errored", func() {
							Ω(fakeSchedulerDB.FinishBuildCallCount()).Should(Equal(1))

							buildID, status := fakeSchedulerDB.FinishBuildArgsForCall(0)
							Ω(buildID).Should(Equal(pendingBuild.ID))
							Ω(status).Should(Equal(db.StatusErrored))
						})
					})

					Context("when the inputs are not yet determined", func() {
						BeforeEach(func() {
							fakeSchedulerDB.GetNextPendingBuildReturns(pendingBuild, []db.BuildInput{}, nil)

							fakeSchedulerDB.GetLatestInputVersionsReturns(pendingInputs, nil)
						})

						It("scans for new versions for each input, and queries for the latest job inputs", func() {
							Ω(fakeScanner.ScanCallCount()).Should(Equal(2))

							_, resourceName := fakeScanner.ScanArgsForCall(0)
							Ω(resourceName).Should(Equal("some-resource"))

							_, resourceName = fakeScanner.ScanArgsForCall(1)
							Ω(resourceName).Should(Equal("some-other-resource"))

							Ω(fakeSchedulerDB.GetLatestInputVersionsCallCount()).Should(Equal(1))
							inputConfigs := fakeSchedulerDB.GetLatestInputVersionsArgsForCall(0)
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
			})

			Context("when the build cannot be scheduled", func() {
				BeforeEach(func() {
					fakeSchedulerDB.ScheduleBuildReturns(false, nil)
				})

				It("does not start a build", func() {
					Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
				})

				Context("and the build's inputs are not determined", func() {
					BeforeEach(func() {
						fakeSchedulerDB.GetNextPendingBuildReturns(pendingBuild, []db.BuildInput{}, nil)
					})

					It("does not perform any scans", func() {
						Ω(fakeScanner.ScanCallCount()).Should(Equal(0))
					})
				})
			})
		})

		Context("when a pending build is not found", func() {
			BeforeEach(func() {
				fakeSchedulerDB.GetNextPendingBuildReturns(db.Build{}, []db.BuildInput{}, db.ErrNoBuild)
			})

			It("does not start a build", func() {
				scheduler.TryNextPendingBuild(logger, job, resources)
				Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
			})
		})

		Context("when getting the next pending build fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeSchedulerDB.GetNextPendingBuildReturns(db.Build{}, []db.BuildInput{}, disaster)
			})

			It("does not start a build", func() {
				scheduler.TryNextPendingBuild(logger, job, resources)
				Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
			})
		})
	})

	Describe("TriggerImmediately", func() {
		It("creates a build without any specific inputs", func() {
			_, err := scheduler.TriggerImmediately(logger, job, resources)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeSchedulerDB.GetLatestInputVersionsCallCount()).Should(Equal(0))

			Ω(fakeSchedulerDB.CreateJobBuildCallCount()).Should(Equal(1))

			jobName := fakeSchedulerDB.CreateJobBuildArgsForCall(0)
			Ω(jobName).Should(Equal("some-job"))
		})

		Context("when creating the build succeeds", func() {
			BeforeEach(func() {
				fakeSchedulerDB.CreateJobBuildReturns(db.Build{ID: 128, Name: "42"}, nil)
			})

			Context("and it can be scheduled", func() {
				BeforeEach(func() {
					fakeSchedulerDB.ScheduleBuildReturns(true, nil)
				})

				Context("and creating the engine build succeeds", func() {
					var createdBuild *enginefakes.FakeBuild

					BeforeEach(func() {
						createdBuild = new(enginefakes.FakeBuild)
						fakeEngine.CreateBuildReturns(createdBuild, nil)
					})

					It("triggers a build of the job with the found inputs", func() {
						build, err := scheduler.TriggerImmediately(logger, job, resources)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(build).Should(Equal(db.Build{ID: 128, Name: "42"}))

						Eventually(fakeSchedulerDB.ScheduleBuildCallCount).Should(Equal(1))
						scheduledBuildID, serial := fakeSchedulerDB.ScheduleBuildArgsForCall(0)
						Ω(scheduledBuildID).Should(Equal(128))
						Ω(serial).Should(Equal(job.Serial))

						Eventually(factory.CreateCallCount).Should(Equal(1))
						createJob, createResources, createInputs := factory.CreateArgsForCall(0)
						Ω(createJob).Should(Equal(job))
						Ω(createResources).Should(Equal(resources))
						Ω(createInputs).Should(BeZero())

						Eventually(fakeEngine.CreateBuildCallCount).Should(Equal(1))
						builtBuild, plan := fakeEngine.CreateBuildArgsForCall(0)
						Ω(builtBuild).Should(Equal(db.Build{ID: 128, Name: "42"}))
						Ω(plan).Should(Equal(createdPlan))
					})

					It("immediately resumes the build", func() {
						build, err := scheduler.TriggerImmediately(logger, job, resources)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(build).Should(Equal(db.Build{ID: 128, Name: "42"}))

						Eventually(createdBuild.ResumeCallCount).Should(Equal(1))
					})
				})
			})

			Context("when the build cannot be scheduled", func() {
				BeforeEach(func() {
					fakeSchedulerDB.ScheduleBuildReturns(false, nil)
				})

				It("does not start a build", func() {
					_, err := scheduler.TriggerImmediately(logger, job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
				})
			})
		})

		Context("when creating the build fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeSchedulerDB.CreateJobBuildReturns(db.Build{}, disaster)
			})

			It("returns the error", func() {
				_, err := scheduler.TriggerImmediately(logger, job, resources)
				Ω(err).Should(Equal(disaster))
			})

			It("does not start a build", func() {
				scheduler.TriggerImmediately(logger, job, resources)
				Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(0))
			})
		})
	})
})
