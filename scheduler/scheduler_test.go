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

		Context("when inputs new found", func() {
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

				Ω(db.GetBuildForInputsCallCount()).Should(Equal(1))

				checkedJob, checkedInputs := db.GetBuildForInputsArgsForCall(0)
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

					Ω(db.GetBuildForInputsCallCount()).Should(Equal(1))

					checkedJob, checkedInputs := db.GetBuildForInputsArgsForCall(0)
					Ω(checkedJob).Should(Equal("some-job"))
					Ω(checkedInputs).Should(Equal(foundInputs))
				})
			})

			Context("and they are not used for a build", func() {
				BeforeEach(func() {
					db.GetBuildForInputsReturns(builds.Build{}, errors.New("no build"))
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
						db.CreateBuildWithInputsReturns(builds.Build{ID: 42}, nil)
					})

					It("triggers a build of the job with the found inputs", func() {
						err := scheduler.BuildLatestInputs(job)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(builder.BuildCallCount()).Should(Equal(1))

						builtBuild, builtJob, builtInputs := builder.BuildArgsForCall(0)
						Ω(builtBuild).Should(Equal(builds.Build{ID: 42}))
						Ω(builtJob).Should(Equal(job))
						Ω(builtInputs).Should(Equal(foundInputs))
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
					db.GetBuildForInputsReturns(builds.Build{ID: 42}, nil)
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
				db.GetNextPendingBuildReturns(builds.Build{ID: 42}, pendingInputs, nil)
			})

			It("builds it", func() {
				err := scheduler.TryNextPendingBuild(job)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(builder.BuildCallCount()).Should(Equal(1))

				builtBuild, builtJob, builtInputs := builder.BuildArgsForCall(0)
				Ω(builtBuild).Should(Equal(builds.Build{ID: 42}))
				Ω(builtJob).Should(Equal(job))
				Ω(builtInputs).Should(Equal(pendingInputs))
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
})
