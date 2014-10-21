package scheduler_test

import (
	"errors"

	"github.com/concourse/atc/builder/fakebuilder"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	. "github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/fakes"
	tbuilds "github.com/concourse/turbine/api/builds"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scheduler", func() {
	var (
		db      *fakes.FakeSchedulerDB
		factory *fakes.FakeBuildFactory
		builder *fakebuilder.FakeBuilder
		tracker *fakes.FakeBuildTracker

		createdTurbineBuild tbuilds.Build

		job config.Job

		scheduler *Scheduler
	)

	BeforeEach(func() {
		db = new(fakes.FakeSchedulerDB)
		factory = new(fakes.FakeBuildFactory)
		builder = new(fakebuilder.FakeBuilder)
		tracker = new(fakes.FakeBuildTracker)

		createdTurbineBuild = tbuilds.Build{
			Config: tbuilds.Config{
				Run: tbuilds.RunConfig{Path: "some-build"},
			},
		}

		factory.CreateReturns(createdTurbineBuild, nil)

		scheduler = &Scheduler{
			Logger:  lagertest.NewTestLogger("test"),
			DB:      db,
			Factory: factory,
			Builder: builder,
			Tracker: tracker,
		}

		job = config.Job{
			Name: "some-job",

			Serial: true,

			Inputs: []config.Input{
				{
					Resource: "some-resource",
					Params:   config.Params{"some": "params"},
				},
				{
					Resource: "some-other-resource",
					Params:   config.Params{"some": "params"},
				},
			},
		}
	})

	Describe("TrackInFlightBuilds", func() {
		var inFlightBuilds []builds.Build

		BeforeEach(func() {
			inFlightBuilds = []builds.Build{
				{ID: 1},
				{ID: 2},
				{ID: 3},
			}

			db.GetAllStartedBuildsReturns(inFlightBuilds, nil)
		})

		It("invokes the tracker with all currently in-flight builds", func() {
			err := scheduler.TrackInFlightBuilds()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(tracker.TrackBuildCallCount()).Should(Equal(3))
			Ω(tracker.TrackBuildArgsForCall(0)).Should(Equal(inFlightBuilds[0]))
			Ω(tracker.TrackBuildArgsForCall(1)).Should(Equal(inFlightBuilds[1]))
			Ω(tracker.TrackBuildArgsForCall(2)).Should(Equal(inFlightBuilds[2]))
		})
	})

	Describe("BuildLatestInputs", func() {
		Context("when no inputs are available", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				db.GetLatestInputVersionsReturns(nil, disaster)
			})

			It("returns the error", func() {
				err := scheduler.BuildLatestInputs(job)
				Ω(err).Should(Equal(disaster))
			})

			It("does not trigger a build", func() {
				scheduler.BuildLatestInputs(job)

				Ω(builder.BuildCallCount()).Should(Equal(0))
			})
		})

		Context("when the job has no inputs", func() {
			BeforeEach(func() {
				job.Inputs = []config.Input{}
			})

			It("succeeds", func() {
				err := scheduler.BuildLatestInputs(job)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("does not try to fetch inputs from the database", func() {
				scheduler.BuildLatestInputs(job)

				Ω(db.GetLatestInputVersionsCallCount()).Should(BeZero())
			})

			It("does not trigger a build", func() {
				scheduler.BuildLatestInputs(job)

				Ω(builder.BuildCallCount()).Should(Equal(0))
			})
		})

		Context("when inputs are found", func() {
			foundInputs := builds.VersionedResources{
				{Name: "some-resource", Version: builds.Version{"version": "1"}},
				{Name: "some-other-resource", Version: builds.Version{"version": "2"}},
			}

			BeforeEach(func() {
				db.GetLatestInputVersionsReturns(foundInputs, nil)
			})

			It("checks if they are already used for a build", func() {
				err := scheduler.BuildLatestInputs(job)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(db.GetJobBuildForInputsCallCount()).Should(Equal(1))

				checkedJob, checkedInputs := db.GetJobBuildForInputsArgsForCall(0)
				Ω(checkedJob).Should(Equal("some-job"))
				Ω(checkedInputs).Should(Equal(foundInputs))
			})

			Context("and the job has inputs configured not to check", func() {
				BeforeEach(func() {
					trigger := false

					job.Inputs = append(job.Inputs, config.Input{
						Resource: "some-non-checking-resource",
						Trigger:  &trigger,
					})

					foundInputsWithCheck := append(
						foundInputs,
						builds.VersionedResource{
							Name:    "some-non-checking-resource",
							Version: builds.Version{"version": 3},
						},
					)

					db.GetLatestInputVersionsReturns(foundInputsWithCheck, nil)
				})

				It("excludes them from the inputs when checking for a build", func() {
					err := scheduler.BuildLatestInputs(job)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(db.GetJobBuildForInputsCallCount()).Should(Equal(1))

					checkedJob, checkedInputs := db.GetJobBuildForInputsArgsForCall(0)
					Ω(checkedJob).Should(Equal("some-job"))
					Ω(checkedInputs).Should(Equal(foundInputs))
				})
			})

			Context("and all inputs are configured not to check", func() {
				BeforeEach(func() {
					trigger := false

					for i, input := range job.Inputs {
						noChecking := input
						noChecking.Trigger = &trigger

						job.Inputs[i] = noChecking
					}
				})

				It("does not check for builds for the inputs", func() {
					err := scheduler.BuildLatestInputs(job)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(db.GetJobBuildForInputsCallCount()).Should(Equal(0))
				})

				It("does not create a build", func() {
					err := scheduler.BuildLatestInputs(job)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(db.CreateJobBuildWithInputsCallCount()).Should(Equal(0))
				})

				It("does not trigger a build", func() {
					err := scheduler.BuildLatestInputs(job)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(builder.BuildCallCount()).Should(Equal(0))
				})
			})

			Context("and they are not used for a build", func() {
				BeforeEach(func() {
					db.GetJobBuildForInputsReturns(builds.Build{}, errors.New("no build"))
				})

				It("creates a build with the found inputs", func() {
					err := scheduler.BuildLatestInputs(job)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(db.CreateJobBuildWithInputsCallCount()).Should(Equal(1))
					buildJob, buildInputs := db.CreateJobBuildWithInputsArgsForCall(0)
					Ω(buildJob).Should(Equal("some-job"))
					Ω(buildInputs).Should(Equal(foundInputs))
				})

				Context("when creating the build succeeds", func() {
					BeforeEach(func() {
						db.CreateJobBuildWithInputsReturns(builds.Build{ID: 128, Name: "42"}, nil)
					})

					Context("and it can be scheduled", func() {
						BeforeEach(func() {
							db.ScheduleBuildReturns(true, nil)
						})

						It("triggers a build of the job with the found inputs", func() {
							err := scheduler.BuildLatestInputs(job)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(db.ScheduleBuildCallCount()).Should(Equal(1))
							scheduledBuildID, serial := db.ScheduleBuildArgsForCall(0)
							Ω(scheduledBuildID).Should(Equal(128))
							Ω(serial).Should(Equal(job.Serial))

							Ω(factory.CreateCallCount()).Should(Equal(1))
							createJob, createInputs := factory.CreateArgsForCall(0)
							Ω(createJob).Should(Equal(job))
							Ω(createInputs).Should(Equal(foundInputs))

							Ω(builder.BuildCallCount()).Should(Equal(1))
							builtBuild, builtTurbineBuild := builder.BuildArgsForCall(0)
							Ω(builtBuild).Should(Equal(builds.Build{ID: 128, Name: "42"}))
							Ω(builtTurbineBuild).Should(Equal(createdTurbineBuild))
						})
					})

					Context("when the build cannot be scheduled", func() {
						BeforeEach(func() {
							db.ScheduleBuildReturns(false, nil)
						})

						It("does not start a build", func() {
							err := scheduler.BuildLatestInputs(job)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(builder.BuildCallCount()).Should(Equal(0))
						})
					})
				})

				Context("when creating the build fails", func() {
					disaster := errors.New("oh no!")

					BeforeEach(func() {
						db.CreateJobBuildWithInputsReturns(builds.Build{}, disaster)
					})

					It("returns the error", func() {
						err := scheduler.BuildLatestInputs(job)
						Ω(err).Should(Equal(disaster))
					})

					It("does not start a build", func() {
						scheduler.BuildLatestInputs(job)
						Ω(builder.BuildCallCount()).Should(Equal(0))
					})
				})
			})

			Context("but they are already used for a build", func() {
				BeforeEach(func() {
					db.GetJobBuildForInputsReturns(builds.Build{ID: 128, Name: "42"}, nil)
				})

				It("does not trigger a build", func() {
					err := scheduler.BuildLatestInputs(job)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(builder.BuildCallCount()).Should(Equal(0))
				})
			})
		})
	})

	Describe("TryNextPendingBuild", func() {
		Context("when a pending build is found", func() {
			pendingInputs := builds.VersionedResources{
				{Name: "some-resource", Version: builds.Version{"version": "1"}},
				{Name: "some-other-resource", Version: builds.Version{"version": "2"}},
			}

			BeforeEach(func() {
				db.GetNextPendingBuildReturns(builds.Build{ID: 128, Name: "42"}, pendingInputs, nil)
			})

			Context("and it can be scheduled", func() {
				BeforeEach(func() {
					db.ScheduleBuildReturns(true, nil)
				})

				It("builds it", func() {
					err := scheduler.TryNextPendingBuild(job)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(db.ScheduleBuildCallCount()).Should(Equal(1))
					scheduledBuildID, serial := db.ScheduleBuildArgsForCall(0)
					Ω(scheduledBuildID).Should(Equal(128))
					Ω(serial).Should(Equal(job.Serial))

					Ω(factory.CreateCallCount()).Should(Equal(1))
					createJob, createInputs := factory.CreateArgsForCall(0)
					Ω(createJob).Should(Equal(job))
					Ω(createInputs).Should(Equal(pendingInputs))

					Ω(builder.BuildCallCount()).Should(Equal(1))
					builtBuild, builtTurbineBuild := builder.BuildArgsForCall(0)
					Ω(builtBuild).Should(Equal(builds.Build{ID: 128, Name: "42"}))
					Ω(builtTurbineBuild).Should(Equal(createdTurbineBuild))
				})
			})

			Context("when the build cannot be scheduled", func() {
				BeforeEach(func() {
					db.ScheduleBuildReturns(false, nil)
				})

				It("does not start a build", func() {
					err := scheduler.TryNextPendingBuild(job)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(builder.BuildCallCount()).Should(Equal(0))
				})
			})
		})

		Context("when a pending build is not found", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				db.GetNextPendingBuildReturns(builds.Build{}, builds.VersionedResources{}, disaster)
			})

			It("returns the error", func() {
				err := scheduler.TryNextPendingBuild(job)
				Ω(err).Should(Equal(disaster))
			})

			It("does not start a build", func() {
				scheduler.TryNextPendingBuild(job)
				Ω(builder.BuildCallCount()).Should(Equal(0))
			})
		})
	})

	Describe("TriggerImmediately", func() {
		Context("when the job does not have any dependant inputs", func() {
			It("creates a build without any specific inputs", func() {
				_, err := scheduler.TriggerImmediately(job)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(db.GetLatestInputVersionsCallCount()).Should(Equal(0))

				Ω(db.CreateJobBuildWithInputsCallCount()).Should(Equal(1))

				jobName, inputs := db.CreateJobBuildWithInputsArgsForCall(0)
				Ω(jobName).Should(Equal("some-job"))
				Ω(inputs).Should(BeZero())
			})

			Context("when creating the build succeeds", func() {
				BeforeEach(func() {
					db.CreateJobBuildWithInputsReturns(builds.Build{ID: 128, Name: "42"}, nil)
				})

				Context("and it can be scheduled", func() {
					BeforeEach(func() {
						db.ScheduleBuildReturns(true, nil)
					})

					It("triggers a build of the job with the found inputs", func() {
						build, err := scheduler.TriggerImmediately(job)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(build).Should(Equal(builds.Build{ID: 128, Name: "42"}))

						Ω(db.ScheduleBuildCallCount()).Should(Equal(1))
						scheduledBuildID, serial := db.ScheduleBuildArgsForCall(0)
						Ω(scheduledBuildID).Should(Equal(128))
						Ω(serial).Should(Equal(job.Serial))

						Ω(factory.CreateCallCount()).Should(Equal(1))
						createJob, createInputs := factory.CreateArgsForCall(0)
						Ω(createJob).Should(Equal(job))
						Ω(createInputs).Should(BeZero())

						Ω(builder.BuildCallCount()).Should(Equal(1))
						builtBuild, builtTurbineBuild := builder.BuildArgsForCall(0)
						Ω(builtBuild).Should(Equal(builds.Build{ID: 128, Name: "42"}))
						Ω(builtTurbineBuild).Should(Equal(createdTurbineBuild))
					})
				})

				Context("when the build cannot be scheduled", func() {
					BeforeEach(func() {
						db.ScheduleBuildReturns(false, nil)
					})

					It("does not start a build", func() {
						_, err := scheduler.TriggerImmediately(job)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(builder.BuildCallCount()).Should(Equal(0))
					})
				})
			})

			Context("when creating the build fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					db.CreateJobBuildWithInputsReturns(builds.Build{}, disaster)
				})

				It("returns the error", func() {
					_, err := scheduler.TriggerImmediately(job)
					Ω(err).Should(Equal(disaster))
				})

				It("does not start a build", func() {
					scheduler.TriggerImmediately(job)
					Ω(builder.BuildCallCount()).Should(Equal(0))
				})
			})
		})

		Context("when the job has dependant inputs", func() {
			BeforeEach(func() {
				job.Inputs = append(job.Inputs, config.Input{
					Resource: "some-dependant-resource",
					Passed:   []string{"job-a"},
				})
			})

			Context("and they can be satisfied", func() {
				foundInputs := builds.VersionedResources{
					{Name: "some-dependant-resource", Version: builds.Version{"version": "2"}},
				}

				BeforeEach(func() {
					db.GetLatestInputVersionsReturns(foundInputs, nil)
				})

				It("creates a build with the found inputs", func() {
					_, err := scheduler.TriggerImmediately(job)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(db.GetLatestInputVersionsCallCount()).Should(Equal(1))
					Ω(db.GetLatestInputVersionsArgsForCall(0)).Should(Equal([]config.Input{
						{
							Resource: "some-dependant-resource",
							Passed:   []string{"job-a"},
						},
					}))

					Ω(db.CreateJobBuildWithInputsCallCount()).Should(Equal(1))

					jobName, inputs := db.CreateJobBuildWithInputsArgsForCall(0)
					Ω(jobName).Should(Equal("some-job"))
					Ω(inputs).Should(Equal(foundInputs))
				})

				Context("when creating the build succeeds", func() {
					BeforeEach(func() {
						db.CreateJobBuildWithInputsReturns(builds.Build{ID: 128, Name: "42"}, nil)
					})

					Context("and it can be scheduled", func() {
						BeforeEach(func() {
							db.ScheduleBuildReturns(true, nil)
						})

						It("triggers a build of the job with the found inputs", func() {
							build, err := scheduler.TriggerImmediately(job)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(build).Should(Equal(builds.Build{ID: 128, Name: "42"}))

							Ω(db.ScheduleBuildCallCount()).Should(Equal(1))
							scheduledBuildID, serial := db.ScheduleBuildArgsForCall(0)
							Ω(scheduledBuildID).Should(Equal(128))
							Ω(serial).Should(Equal(job.Serial))

							Ω(factory.CreateCallCount()).Should(Equal(1))
							createJob, createInputs := factory.CreateArgsForCall(0)
							Ω(createJob).Should(Equal(job))
							Ω(createInputs).Should(Equal(foundInputs))

							Ω(builder.BuildCallCount()).Should(Equal(1))
							builtBuild, builtTurbineBuild := builder.BuildArgsForCall(0)
							Ω(builtBuild).Should(Equal(builds.Build{ID: 128, Name: "42"}))
							Ω(builtTurbineBuild).Should(Equal(createdTurbineBuild))
						})
					})
				})

				Context("when the build cannot be scheduled", func() {
					BeforeEach(func() {
						db.ScheduleBuildReturns(false, nil)
					})

					It("does not start a build", func() {
						scheduler.TriggerImmediately(job)
						Ω(builder.BuildCallCount()).Should(Equal(0))
					})
				})

				Context("when creating the build fails", func() {
					disaster := errors.New("oh no!")

					BeforeEach(func() {
						db.CreateJobBuildWithInputsReturns(builds.Build{}, disaster)
					})

					It("returns the error", func() {
						_, err := scheduler.TriggerImmediately(job)
						Ω(err).Should(Equal(disaster))
					})

					It("does not start a build", func() {
						scheduler.TriggerImmediately(job)
						Ω(builder.BuildCallCount()).Should(Equal(0))
					})
				})
			})

			Context("but they cannot be satisfied", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					db.GetLatestInputVersionsReturns(nil, disaster)
				})

				It("returns the error", func() {
					_, err := scheduler.TriggerImmediately(job)
					Ω(err).Should(Equal(disaster))
				})

				It("does not create or start a build", func() {
					scheduler.TriggerImmediately(job)

					Ω(db.CreateJobBuildWithInputsCallCount()).Should(Equal(0))

					Ω(builder.BuildCallCount()).Should(Equal(0))
				})
			})
		})
	})
})
