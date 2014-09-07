package scheduler_test

import (
	"errors"

	"github.com/concourse/atc/builder/fakebuilder"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	. "github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/fakes"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scheduler", func() {
	var db *fakes.FakeSchedulerDB
	var builder *fakebuilder.FakeBuilder

	var job config.Job

	var scheduler *Scheduler

	BeforeEach(func() {
		db = new(fakes.FakeSchedulerDB)
		builder = new(fakebuilder.FakeBuilder)

		scheduler = &Scheduler{
			DB:      db,
			Builder: builder,
			Logger:  lagertest.NewTestLogger("test"),
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
					job.Inputs = append(job.Inputs, config.Input{
						Resource:  "some-non-checking-resource",
						DontCheck: true,
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
					for i, input := range job.Inputs {
						noChecking := input
						noChecking.DontCheck = true

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

					Ω(db.CreateBuildWithInputsCallCount()).Should(Equal(0))
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

					Ω(db.CreateBuildWithInputsCallCount()).Should(Equal(1))

					buildJob, buildInputs := db.CreateBuildWithInputsArgsForCall(0)
					Ω(buildJob).Should(Equal("some-job"))
					Ω(buildInputs).Should(Equal(foundInputs))
				})

				Context("when creating the build succeeds", func() {
					BeforeEach(func() {
						db.CreateBuildWithInputsReturns(builds.Build{Name: "42"}, nil)
					})

					Context("and it can be scheduled", func() {
						BeforeEach(func() {
							db.ScheduleBuildReturns(true, nil)
						})

						It("triggers a build of the job with the found inputs", func() {
							err := scheduler.BuildLatestInputs(job)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(db.ScheduleBuildCallCount()).Should(Equal(1))
							scheduledJob, scheduledBuild, serial := db.ScheduleBuildArgsForCall(0)
							Ω(scheduledJob).Should(Equal("some-job"))
							Ω(scheduledBuild).Should(Equal("42"))
							Ω(serial).Should(Equal(job.Serial))

							Ω(builder.BuildCallCount()).Should(Equal(1))

							builtBuild, builtJob, builtInputs := builder.BuildArgsForCall(0)
							Ω(builtBuild).Should(Equal(builds.Build{Name: "42"}))
							Ω(builtJob).Should(Equal(job))
							Ω(builtInputs).Should(Equal(foundInputs))
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
						db.CreateBuildWithInputsReturns(builds.Build{}, disaster)
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
					db.GetJobBuildForInputsReturns(builds.Build{Name: "42"}, nil)
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
				db.GetNextPendingBuildReturns(builds.Build{Name: "42"}, pendingInputs, nil)
			})

			Context("and it can be scheduled", func() {
				BeforeEach(func() {
					db.ScheduleBuildReturns(true, nil)
				})

				It("builds it", func() {
					err := scheduler.TryNextPendingBuild(job)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(db.ScheduleBuildCallCount()).Should(Equal(1))
					scheduledJob, scheduledBuild, serial := db.ScheduleBuildArgsForCall(0)
					Ω(scheduledJob).Should(Equal("some-job"))
					Ω(scheduledBuild).Should(Equal("42"))
					Ω(serial).Should(Equal(job.Serial))

					Ω(builder.BuildCallCount()).Should(Equal(1))

					builtBuild, builtJob, builtInputs := builder.BuildArgsForCall(0)
					Ω(builtBuild.Name).Should(Equal("42"))
					Ω(builtJob).Should(Equal(job))
					Ω(builtInputs).Should(Equal(pendingInputs))
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

				Ω(db.CreateBuildWithInputsCallCount()).Should(Equal(1))

				jobName, inputs := db.CreateBuildWithInputsArgsForCall(0)
				Ω(jobName).Should(Equal("some-job"))
				Ω(inputs).Should(BeZero())
			})

			Context("when creating the build succeeds", func() {
				BeforeEach(func() {
					db.CreateBuildWithInputsReturns(builds.Build{Name: "42"}, nil)
				})

				Context("and it can be scheduled", func() {
					BeforeEach(func() {
						db.ScheduleBuildReturns(true, nil)
					})

					It("triggers a build of the job with the found inputs", func() {
						build, err := scheduler.TriggerImmediately(job)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(build).Should(Equal(builds.Build{Name: "42"}))

						Ω(db.ScheduleBuildCallCount()).Should(Equal(1))
						scheduledJob, scheduledBuild, serial := db.ScheduleBuildArgsForCall(0)
						Ω(scheduledJob).Should(Equal("some-job"))
						Ω(scheduledBuild).Should(Equal("42"))
						Ω(serial).Should(Equal(job.Serial))

						Ω(builder.BuildCallCount()).Should(Equal(1))

						builtBuild, builtJob, builtInputs := builder.BuildArgsForCall(0)
						Ω(builtBuild).Should(Equal(builds.Build{Name: "42"}))
						Ω(builtJob).Should(Equal(job))
						Ω(builtInputs).Should(BeZero())
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
					db.CreateBuildWithInputsReturns(builds.Build{}, disaster)
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

					Ω(db.CreateBuildWithInputsCallCount()).Should(Equal(1))

					jobName, inputs := db.CreateBuildWithInputsArgsForCall(0)
					Ω(jobName).Should(Equal("some-job"))
					Ω(inputs).Should(Equal(foundInputs))
				})

				Context("when creating the build succeeds", func() {
					BeforeEach(func() {
						db.CreateBuildWithInputsReturns(builds.Build{Name: "42"}, nil)
					})

					Context("and it can be scheduled", func() {
						BeforeEach(func() {
							db.ScheduleBuildReturns(true, nil)
						})

						It("triggers a build of the job with the found inputs", func() {
							build, err := scheduler.TriggerImmediately(job)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(build).Should(Equal(builds.Build{Name: "42"}))

							Ω(db.ScheduleBuildCallCount()).Should(Equal(1))
							scheduledJob, scheduledBuild, serial := db.ScheduleBuildArgsForCall(0)
							Ω(scheduledJob).Should(Equal("some-job"))
							Ω(scheduledBuild).Should(Equal("42"))
							Ω(serial).Should(Equal(job.Serial))

							Ω(builder.BuildCallCount()).Should(Equal(1))

							builtBuild, builtJob, builtInputs := builder.BuildArgsForCall(0)
							Ω(builtBuild).Should(Equal(builds.Build{Name: "42"}))
							Ω(builtJob).Should(Equal(job))
							Ω(builtInputs).Should(Equal(foundInputs))
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
						db.CreateBuildWithInputsReturns(builds.Build{}, disaster)
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

					Ω(db.CreateBuildWithInputsCallCount()).Should(Equal(0))

					Ω(builder.BuildCallCount()).Should(Equal(0))
				})
			})
		})
	})
})
