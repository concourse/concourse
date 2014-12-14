package scheduler_test

import (
	"database/sql"
	"errors"

	"github.com/concourse/atc"
	"github.com/concourse/atc/builder/fakebuilder"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	"github.com/concourse/atc/engine"
	. "github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/fakes"
	"github.com/concourse/turbine"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scheduler", func() {
	var (
		schedulerDB *fakes.FakeSchedulerDB
		factory     *fakes.FakeBuildFactory
		builder     *fakebuilder.FakeBuilder
		locker      *fakes.FakeLocker
		tracker     *fakes.FakeBuildTracker

		createdBuildPlan engine.BuildPlan

		job       atc.JobConfig
		resources atc.ResourceConfigs

		readLock *dbfakes.FakeLock

		scheduler *Scheduler
	)

	BeforeEach(func() {
		schedulerDB = new(fakes.FakeSchedulerDB)
		factory = new(fakes.FakeBuildFactory)
		builder = new(fakebuilder.FakeBuilder)
		locker = new(fakes.FakeLocker)
		tracker = new(fakes.FakeBuildTracker)

		createdBuildPlan = engine.BuildPlan{
			Config: turbine.Config{
				Run: turbine.RunConfig{Path: "some-build"},
			},
		}

		factory.CreateReturns(createdBuildPlan, nil)

		scheduler = &Scheduler{
			Logger:  lagertest.NewTestLogger("test"),
			DB:      schedulerDB,
			Locker:  locker,
			Factory: factory,
			Builder: builder,
			Tracker: tracker,
		}

		yes := true
		job = atc.JobConfig{
			Name: "some-job",

			Serial: true,

			Inputs: []atc.InputConfig{
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

		readLock = new(dbfakes.FakeLock)
		locker.AcquireReadLockReturns(readLock, nil)
	})

	Describe("TrackInFlightBuilds", func() {
		var inFlightBuilds []db.Build

		BeforeEach(func() {
			inFlightBuilds = []db.Build{
				{ID: 1},
				{ID: 2},
				{ID: 3},
			}

			schedulerDB.GetAllStartedBuildsReturns(inFlightBuilds, nil)
		})

		It("invokes the tracker with all currently in-flight builds", func() {
			err := scheduler.TrackInFlightBuilds()
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(tracker.TrackBuildCallCount).Should(Equal(3))
			Ω(tracker.TrackBuildArgsForCall(0)).Should(Equal(inFlightBuilds[0]))
			Ω(tracker.TrackBuildArgsForCall(1)).Should(Equal(inFlightBuilds[1]))
			Ω(tracker.TrackBuildArgsForCall(2)).Should(Equal(inFlightBuilds[2]))
		})
	})

	Describe("BuildLatestInputs", func() {
		Context("when no inputs are available", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				schedulerDB.GetLatestInputVersionsReturns(nil, disaster)
			})

			It("returns the error", func() {
				err := scheduler.BuildLatestInputs(job, resources)
				Ω(err).Should(Equal(disaster))
			})

			It("does not trigger a build", func() {
				scheduler.BuildLatestInputs(job, resources)

				Ω(builder.BuildCallCount()).Should(Equal(0))
			})
		})

		Context("when the job has no inputs", func() {
			BeforeEach(func() {
				job.Inputs = []atc.InputConfig{}
			})

			It("succeeds", func() {
				err := scheduler.BuildLatestInputs(job, resources)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("does not try to fetch inputs from the database", func() {
				scheduler.BuildLatestInputs(job, resources)

				Ω(schedulerDB.GetLatestInputVersionsCallCount()).Should(BeZero())
			})

			It("does not trigger a build", func() {
				scheduler.BuildLatestInputs(job, resources)

				Ω(builder.BuildCallCount()).Should(Equal(0))
			})
		})

		Context("when versions are found", func() {
			foundVersions := db.VersionedResources{
				{Resource: "some-resource", Version: db.Version{"version": "1"}},
				{Resource: "some-other-resource", Version: db.Version{"version": "2"}},
			}

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
				schedulerDB.GetLatestInputVersionsReturns(foundVersions, nil)
			})

			It("checks if they are already used for a build", func() {
				err := scheduler.BuildLatestInputs(job, resources)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(schedulerDB.GetJobBuildForInputsCallCount()).Should(Equal(1))

				checkedJob, checkedInputs := schedulerDB.GetJobBuildForInputsArgsForCall(0)
				Ω(checkedJob).Should(Equal("some-job"))
				Ω(checkedInputs).Should(ConsistOf(newInputs))
			})

			Describe("getting the latest inputs from the database", func() {
				BeforeEach(func() {
					schedulerDB.GetLatestInputVersionsStub = func(inputs []atc.InputConfig) (db.VersionedResources, error) {
						Ω(locker.AcquireReadLockCallCount()).Should(Equal(1))
						Ω(locker.AcquireReadLockArgsForCall(0)).Should(ConsistOf([]db.NamedLock{
							db.ResourceLock("some-resource"),
							db.ResourceLock("some-other-resource"),
						}))

						return foundVersions, nil
					}
				})

				It("is done while holding a read lock for every resource", func() {
					err := scheduler.BuildLatestInputs(job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					// assertion is in stub to guarantee order
					Ω(schedulerDB.GetLatestInputVersionsCallCount()).Should(Equal(1))

					Ω(readLock.ReleaseCallCount()).Should(Equal(1))
				})
			})

			Context("and the job has inputs configured not to check", func() {
				BeforeEach(func() {
					trigger := false

					job.Inputs = append(job.Inputs, atc.InputConfig{
						Resource:   "some-non-checking-resource",
						RawTrigger: &trigger,
					})

					foundVersionsWithCheck := append(
						foundVersions,
						db.VersionedResource{
							Resource: "some-non-checking-resource",
							Version:  db.Version{"version": 3},
						},
					)

					schedulerDB.GetLatestInputVersionsReturns(foundVersionsWithCheck, nil)
				})

				It("excludes them from the inputs when checking for a build", func() {
					err := scheduler.BuildLatestInputs(job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(schedulerDB.GetJobBuildForInputsCallCount()).Should(Equal(1))

					checkedJob, checkedInputs := schedulerDB.GetJobBuildForInputsArgsForCall(0)
					Ω(checkedJob).Should(Equal("some-job"))
					Ω(checkedInputs).Should(Equal(newInputs))
				})
			})

			Context("and all inputs are configured not to check", func() {
				BeforeEach(func() {
					trigger := false

					for i, input := range job.Inputs {
						noChecking := input
						noChecking.RawTrigger = &trigger

						job.Inputs[i] = noChecking
					}
				})

				It("does not check for builds for the inputs", func() {
					err := scheduler.BuildLatestInputs(job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(schedulerDB.GetJobBuildForInputsCallCount()).Should(Equal(0))
				})

				It("does not create a build", func() {
					err := scheduler.BuildLatestInputs(job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(schedulerDB.CreateJobBuildWithInputsCallCount()).Should(Equal(0))
				})

				It("does not trigger a build", func() {
					err := scheduler.BuildLatestInputs(job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(builder.BuildCallCount()).Should(Equal(0))
				})
			})

			Context("and they are not used for a build", func() {
				BeforeEach(func() {
					schedulerDB.GetJobBuildForInputsReturns(db.Build{}, errors.New("no build"))
				})

				It("creates a build with the found inputs", func() {
					err := scheduler.BuildLatestInputs(job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(schedulerDB.CreateJobBuildWithInputsCallCount()).Should(Equal(1))
					buildJob, buildInputs := schedulerDB.CreateJobBuildWithInputsArgsForCall(0)
					Ω(buildJob).Should(Equal("some-job"))
					Ω(buildInputs).Should(Equal(newInputs))
				})

				Context("when creating the build succeeds", func() {
					BeforeEach(func() {
						schedulerDB.CreateJobBuildWithInputsReturns(db.Build{ID: 128, Name: "42"}, nil)
					})

					Context("and it can be scheduled", func() {
						BeforeEach(func() {
							schedulerDB.ScheduleBuildReturns(true, nil)
						})

						It("triggers a build of the job with the found inputs", func() {
							err := scheduler.BuildLatestInputs(job, resources)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(schedulerDB.ScheduleBuildCallCount()).Should(Equal(1))
							scheduledBuildID, serial := schedulerDB.ScheduleBuildArgsForCall(0)
							Ω(scheduledBuildID).Should(Equal(128))
							Ω(serial).Should(Equal(job.Serial))

							Ω(factory.CreateCallCount()).Should(Equal(1))
							createJob, createResources, createInputs := factory.CreateArgsForCall(0)
							Ω(createJob).Should(Equal(job))
							Ω(createResources).Should(Equal(resources))
							Ω(createInputs).Should(Equal(newInputs))

							Ω(builder.BuildCallCount()).Should(Equal(1))
							builtBuild, builtTurbineBuild := builder.BuildArgsForCall(0)
							Ω(builtBuild).Should(Equal(db.Build{ID: 128, Name: "42"}))
							Ω(builtTurbineBuild).Should(Equal(createdBuildPlan))
						})
					})

					Context("when the build cannot be scheduled", func() {
						BeforeEach(func() {
							schedulerDB.ScheduleBuildReturns(false, nil)
						})

						It("does not start a build", func() {
							err := scheduler.BuildLatestInputs(job, resources)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(builder.BuildCallCount()).Should(Equal(0))
						})
					})
				})

				Context("when creating the build fails", func() {
					disaster := errors.New("oh no!")

					BeforeEach(func() {
						schedulerDB.CreateJobBuildWithInputsReturns(db.Build{}, disaster)
					})

					It("returns the error", func() {
						err := scheduler.BuildLatestInputs(job, resources)
						Ω(err).Should(Equal(disaster))
					})

					It("does not start a build", func() {
						scheduler.BuildLatestInputs(job, resources)
						Ω(builder.BuildCallCount()).Should(Equal(0))
					})
				})
			})

			Context("but they are already used for a build", func() {
				BeforeEach(func() {
					schedulerDB.GetJobBuildForInputsReturns(db.Build{ID: 128, Name: "42"}, nil)
				})

				It("does not trigger a build", func() {
					err := scheduler.BuildLatestInputs(job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(builder.BuildCallCount()).Should(Equal(0))
				})
			})
		})
	})

	Describe("TryNextPendingBuild", func() {
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

			BeforeEach(func() {
				schedulerDB.GetNextPendingBuildReturns(db.Build{ID: 128, Name: "42"}, pendingInputs, nil)
			})

			Context("and it can be scheduled", func() {
				BeforeEach(func() {
					schedulerDB.ScheduleBuildReturns(true, nil)
				})

				It("builds it", func() {
					err := scheduler.TryNextPendingBuild(job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(schedulerDB.ScheduleBuildCallCount()).Should(Equal(1))
					scheduledBuildID, serial := schedulerDB.ScheduleBuildArgsForCall(0)
					Ω(scheduledBuildID).Should(Equal(128))
					Ω(serial).Should(Equal(job.Serial))

					Ω(factory.CreateCallCount()).Should(Equal(1))
					createJob, createResources, createInputs := factory.CreateArgsForCall(0)
					Ω(createJob).Should(Equal(job))
					Ω(createResources).Should(Equal(resources))
					Ω(createInputs).Should(Equal(pendingInputs))

					Ω(builder.BuildCallCount()).Should(Equal(1))
					builtBuild, builtTurbineBuild := builder.BuildArgsForCall(0)
					Ω(builtBuild).Should(Equal(db.Build{ID: 128, Name: "42"}))
					Ω(builtTurbineBuild).Should(Equal(createdBuildPlan))
				})
			})

			Context("when the build cannot be scheduled", func() {
				BeforeEach(func() {
					schedulerDB.ScheduleBuildReturns(false, nil)
				})

				It("does not start a build", func() {
					err := scheduler.TryNextPendingBuild(job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(builder.BuildCallCount()).Should(Equal(0))
				})
			})
		})

		Context("when a pending build is not found", func() {
			BeforeEach(func() {
				schedulerDB.GetNextPendingBuildReturns(db.Build{}, []db.BuildInput{}, sql.ErrNoRows)
			})

			It("returns no error", func() {
				err := scheduler.TryNextPendingBuild(job, resources)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("does not start a build", func() {
				scheduler.TryNextPendingBuild(job, resources)
				Ω(builder.BuildCallCount()).Should(Equal(0))
			})
		})

		Context("when getting the next pending build fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				schedulerDB.GetNextPendingBuildReturns(db.Build{}, []db.BuildInput{}, disaster)
			})

			It("returns the error", func() {
				err := scheduler.TryNextPendingBuild(job, resources)
				Ω(err).Should(Equal(disaster))
			})

			It("does not start a build", func() {
				scheduler.TryNextPendingBuild(job, resources)
				Ω(builder.BuildCallCount()).Should(Equal(0))
			})
		})
	})

	Describe("TriggerImmediately", func() {
		Context("when the job does not have any dependant inputs", func() {
			It("creates a build without any specific inputs", func() {
				_, err := scheduler.TriggerImmediately(job, resources)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(schedulerDB.GetLatestInputVersionsCallCount()).Should(Equal(0))

				Ω(schedulerDB.CreateJobBuildWithInputsCallCount()).Should(Equal(1))

				jobName, inputs := schedulerDB.CreateJobBuildWithInputsArgsForCall(0)
				Ω(jobName).Should(Equal("some-job"))
				Ω(inputs).Should(BeZero())
			})

			Context("when creating the build succeeds", func() {
				BeforeEach(func() {
					schedulerDB.CreateJobBuildWithInputsReturns(db.Build{ID: 128, Name: "42"}, nil)
				})

				Context("and it can be scheduled", func() {
					BeforeEach(func() {
						schedulerDB.ScheduleBuildReturns(true, nil)
					})

					It("triggers a build of the job with the found inputs", func() {
						build, err := scheduler.TriggerImmediately(job, resources)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(build).Should(Equal(db.Build{ID: 128, Name: "42"}))

						Ω(schedulerDB.ScheduleBuildCallCount()).Should(Equal(1))
						scheduledBuildID, serial := schedulerDB.ScheduleBuildArgsForCall(0)
						Ω(scheduledBuildID).Should(Equal(128))
						Ω(serial).Should(Equal(job.Serial))

						Ω(factory.CreateCallCount()).Should(Equal(1))
						createJob, createResources, createInputs := factory.CreateArgsForCall(0)
						Ω(createJob).Should(Equal(job))
						Ω(createResources).Should(Equal(resources))
						Ω(createInputs).Should(BeZero())

						Ω(builder.BuildCallCount()).Should(Equal(1))
						builtBuild, builtTurbineBuild := builder.BuildArgsForCall(0)
						Ω(builtBuild).Should(Equal(db.Build{ID: 128, Name: "42"}))
						Ω(builtTurbineBuild).Should(Equal(createdBuildPlan))
					})
				})

				Context("when the build cannot be scheduled", func() {
					BeforeEach(func() {
						schedulerDB.ScheduleBuildReturns(false, nil)
					})

					It("does not start a build", func() {
						_, err := scheduler.TriggerImmediately(job, resources)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(builder.BuildCallCount()).Should(Equal(0))
					})
				})
			})

			Context("when creating the build fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					schedulerDB.CreateJobBuildWithInputsReturns(db.Build{}, disaster)
				})

				It("returns the error", func() {
					_, err := scheduler.TriggerImmediately(job, resources)
					Ω(err).Should(Equal(disaster))
				})

				It("does not start a build", func() {
					scheduler.TriggerImmediately(job, resources)
					Ω(builder.BuildCallCount()).Should(Equal(0))
				})
			})
		})

		Context("when the job has dependant inputs", func() {
			BeforeEach(func() {
				job.Inputs = append(job.Inputs, atc.InputConfig{
					RawName:  "some-dependant-input",
					Resource: "some-dependant-resource",
					Passed:   []string{"job-a"},
				})
			})

			Context("and they can be satisfied", func() {
				foundVersions := db.VersionedResources{
					{Resource: "some-dependant-resource", Version: db.Version{"version": "2"}},
				}

				dependantInputs := []db.BuildInput{
					{
						Name: "some-dependant-input",
						VersionedResource: db.VersionedResource{
							Resource: "some-dependant-resource", Version: db.Version{"version": "2"},
						},
					},
				}

				BeforeEach(func() {
					schedulerDB.GetLatestInputVersionsReturns(foundVersions, nil)
				})

				It("creates a build with the found inputs", func() {
					_, err := scheduler.TriggerImmediately(job, resources)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(schedulerDB.GetLatestInputVersionsCallCount()).Should(Equal(1))
					Ω(schedulerDB.GetLatestInputVersionsArgsForCall(0)).Should(Equal([]atc.InputConfig{
						{
							RawName:  "some-dependant-input",
							Resource: "some-dependant-resource",
							Passed:   []string{"job-a"},
						},
					}))

					Ω(schedulerDB.CreateJobBuildWithInputsCallCount()).Should(Equal(1))

					jobName, inputs := schedulerDB.CreateJobBuildWithInputsArgsForCall(0)
					Ω(jobName).Should(Equal("some-job"))
					Ω(inputs).Should(Equal(dependantInputs))
				})

				Context("when creating the build succeeds", func() {
					BeforeEach(func() {
						schedulerDB.CreateJobBuildWithInputsReturns(db.Build{ID: 128, Name: "42"}, nil)
					})

					Context("and it can be scheduled", func() {
						BeforeEach(func() {
							schedulerDB.ScheduleBuildReturns(true, nil)
						})

						It("triggers a build of the job with the found inputs", func() {
							build, err := scheduler.TriggerImmediately(job, resources)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(build).Should(Equal(db.Build{ID: 128, Name: "42"}))

							Ω(schedulerDB.ScheduleBuildCallCount()).Should(Equal(1))
							scheduledBuildID, serial := schedulerDB.ScheduleBuildArgsForCall(0)
							Ω(scheduledBuildID).Should(Equal(128))
							Ω(serial).Should(Equal(job.Serial))

							Ω(factory.CreateCallCount()).Should(Equal(1))
							createJob, createResources, createInputs := factory.CreateArgsForCall(0)
							Ω(createJob).Should(Equal(job))
							Ω(createResources).Should(Equal(resources))
							Ω(createInputs).Should(Equal(dependantInputs))

							Ω(builder.BuildCallCount()).Should(Equal(1))
							builtBuild, builtTurbineBuild := builder.BuildArgsForCall(0)
							Ω(builtBuild).Should(Equal(db.Build{ID: 128, Name: "42"}))
							Ω(builtTurbineBuild).Should(Equal(createdBuildPlan))
						})
					})
				})

				Context("when the build cannot be scheduled", func() {
					BeforeEach(func() {
						schedulerDB.ScheduleBuildReturns(false, nil)
					})

					It("does not start a build", func() {
						scheduler.TriggerImmediately(job, resources)
						Ω(builder.BuildCallCount()).Should(Equal(0))
					})
				})

				Context("when creating the build fails", func() {
					disaster := errors.New("oh no!")

					BeforeEach(func() {
						schedulerDB.CreateJobBuildWithInputsReturns(db.Build{}, disaster)
					})

					It("returns the error", func() {
						_, err := scheduler.TriggerImmediately(job, resources)
						Ω(err).Should(Equal(disaster))
					})

					It("does not start a build", func() {
						scheduler.TriggerImmediately(job, resources)
						Ω(builder.BuildCallCount()).Should(Equal(0))
					})
				})
			})

			Context("but they cannot be satisfied", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					schedulerDB.GetLatestInputVersionsReturns(nil, disaster)
				})

				It("returns the error", func() {
					_, err := scheduler.TriggerImmediately(job, resources)
					Ω(err).Should(Equal(disaster))
				})

				It("does not create or start a build", func() {
					scheduler.TriggerImmediately(job, resources)

					Ω(schedulerDB.CreateJobBuildWithInputsCallCount()).Should(Equal(0))

					Ω(builder.BuildCallCount()).Should(Equal(0))
				})
			})
		})
	})
})
