package dbng_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Job", func() {
	var (
		job      dbng.Job
		pipeline dbng.Pipeline
		team     dbng.Team
	)

	BeforeEach(func() {
		var err error
		team, err = teamFactory.CreateTeam(atc.Team{Name: "some-team"})
		Expect(err).ToNot(HaveOccurred())

		var created bool
		pipeline, created, err = team.SavePipeline("fake-pipeline", atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "some-job",

					Public: true,

					Serial: true,

					SerialGroups: []string{"serial-group"},

					Plan: atc.PlanSequence{
						{
							Put: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
						},
						{
							Get:      "some-input",
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
							Passed:  []string{"job-1", "job-2"},
							Trigger: true,
						},
						{
							Task:           "some-task",
							Privileged:     true,
							TaskConfigPath: "some/config/path.yml",
							TaskConfig: &atc.TaskConfig{
								RootfsURI: "some-image",
							},
						},
					},
				},
				{
					Name: "some-other-job",
				},
				{
					Name:         "other-serial-group-job",
					SerialGroups: []string{"serial-group", "really-different-group"},
				},
				{
					Name:         "different-serial-group-job",
					SerialGroups: []string{"different-serial-group"},
				},
			},
			Resources: atc.ResourceConfigs{
				{
					Name: "some-resource",
					Type: "some-type",
				},
				{
					Name: "some-other-resource",
					Type: "some-type",
				},
			},
		}, dbng.ConfigVersion(0), dbng.PipelineUnpaused)
		Expect(err).ToNot(HaveOccurred())
		Expect(created).To(BeTrue())

		var found bool
		job, found, err = pipeline.Job("some-job")
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
	})

	Describe("Pause and Unpause", func() {
		It("starts out as unpaused", func() {
			Expect(job.Paused()).To(BeFalse())
		})

		It("can be paused", func() {
			err := job.Pause()
			Expect(err).NotTo(HaveOccurred())

			found, err := job.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(job.Paused()).To(BeTrue())
		})

		It("can be unpaused", func() {
			err := job.Unpause()
			Expect(err).NotTo(HaveOccurred())

			found, err := job.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(job.Paused()).To(BeFalse())
		})
	})

	Describe("FinishedAndNextBuild", func() {
		var otherPipeline dbng.Pipeline
		var otherJob dbng.Job

		BeforeEach(func() {
			var created bool
			var err error
			otherPipeline, created, err = team.SavePipeline("other-pipeline", atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "some-job"},
				},
			}, dbng.ConfigVersion(0), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(BeTrue())

			var found bool
			otherJob, found, err = otherPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		It("can report a job's latest running and finished builds", func() {
			finished, next, err := job.FinishedAndNextBuild()
			Expect(err).NotTo(HaveOccurred())

			Expect(next).To(BeNil())
			Expect(finished).To(BeNil())

			finishedBuild, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			err = finishedBuild.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			otherFinishedBuild, err := otherJob.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			err = otherFinishedBuild.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			finished, next, err = job.FinishedAndNextBuild()
			Expect(err).NotTo(HaveOccurred())

			Expect(next).To(BeNil())
			Expect(finished.ID()).To(Equal(finishedBuild.ID()))

			nextBuild, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			started, err := nextBuild.Start("some-engine", "meta")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			otherNextBuild, err := otherJob.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			otherStarted, err := otherNextBuild.Start("some-engine", "meta")
			Expect(err).NotTo(HaveOccurred())
			Expect(otherStarted).To(BeTrue())

			finished, next, err = job.FinishedAndNextBuild()
			Expect(err).NotTo(HaveOccurred())

			Expect(next.ID()).To(Equal(nextBuild.ID()))
			Expect(finished.ID()).To(Equal(finishedBuild.ID()))

			anotherRunningBuild, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			finished, next, err = job.FinishedAndNextBuild()
			Expect(err).NotTo(HaveOccurred())

			Expect(next.ID()).To(Equal(nextBuild.ID())) // not anotherRunningBuild
			Expect(finished.ID()).To(Equal(finishedBuild.ID()))

			started, err = anotherRunningBuild.Start("some-engine", "meta")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			finished, next, err = job.FinishedAndNextBuild()
			Expect(err).NotTo(HaveOccurred())

			Expect(next.ID()).To(Equal(nextBuild.ID())) // not anotherRunningBuild
			Expect(finished.ID()).To(Equal(finishedBuild.ID()))

			err = nextBuild.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			finished, next, err = job.FinishedAndNextBuild()
			Expect(err).NotTo(HaveOccurred())

			Expect(next.ID()).To(Equal(anotherRunningBuild.ID()))
			Expect(finished.ID()).To(Equal(nextBuild.ID()))
		})
	})

	Describe("UpdateFirstLoggedBuildID", func() {
		It("updates FirstLoggedBuildID on a job", func() {
			By("starting out as 0")
			Expect(job.FirstLoggedBuildID()).To(BeZero())

			By("increasing it to 57")
			err := job.UpdateFirstLoggedBuildID(57)
			Expect(err).NotTo(HaveOccurred())

			found, err := job.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(job.FirstLoggedBuildID()).To(Equal(57))

			By("not erroring when it's called with the same number")
			err = job.UpdateFirstLoggedBuildID(57)
			Expect(err).NotTo(HaveOccurred())

			By("erroring when the number decreases")
			err = job.UpdateFirstLoggedBuildID(56)
			Expect(err).To(Equal(dbng.FirstLoggedBuildIDDecreasedError{
				Job:   "some-job",
				OldID: 57,
				NewID: 56,
			}))
		})
	})

	Context("Builds", func() {
		var (
			builds       [10]dbng.Build
			someJob      dbng.Job
			someOtherJob dbng.Job
		)

		BeforeEach(func() {
			for i := 0; i < 10; i++ {
				var found bool
				var err error
				someJob, found, err = pipeline.Job("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				someOtherJob, found, err = pipeline.Job("some-other-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err := someJob.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				_, err = someOtherJob.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				builds[i] = build
			}
		})

		Context("when there are no builds to be found", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := someOtherJob.Builds(dbng.Page{})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]dbng.Build{}))
				Expect(pagination).To(Equal(dbng.Pagination{}))
			})
		})

		Context("with no since/until", func() {
			It("returns the first page, with the given limit, and a next page", func() {
				buildsPage, pagination, err := someJob.Builds(dbng.Page{Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]dbng.Build{builds[9], builds[8]}))
				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(Equal(&dbng.Page{Since: builds[8].ID(), Limit: 2}))
			})
		})

		Context("with a since that places it in the middle of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := someJob.Builds(dbng.Page{Since: builds[6].ID(), Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]dbng.Build{builds[5], builds[4]}))
				Expect(pagination.Previous).To(Equal(&dbng.Page{Until: builds[5].ID(), Limit: 2}))
				Expect(pagination.Next).To(Equal(&dbng.Page{Since: builds[4].ID(), Limit: 2}))
			})
		})

		Context("with a since that places it at the end of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := someJob.Builds(dbng.Page{Since: builds[2].ID(), Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]dbng.Build{builds[1], builds[0]}))
				Expect(pagination.Previous).To(Equal(&dbng.Page{Until: builds[1].ID(), Limit: 2}))
				Expect(pagination.Next).To(BeNil())
			})
		})

		Context("with an until that places it in the middle of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := someJob.Builds(dbng.Page{Until: builds[6].ID(), Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]dbng.Build{builds[8], builds[7]}))
				Expect(pagination.Previous).To(Equal(&dbng.Page{Until: builds[8].ID(), Limit: 2}))
				Expect(pagination.Next).To(Equal(&dbng.Page{Since: builds[7].ID(), Limit: 2}))
			})
		})

		Context("with a until that places it at the beginning of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := someJob.Builds(dbng.Page{Until: builds[7].ID(), Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]dbng.Build{builds[9], builds[8]}))
				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(Equal(&dbng.Page{Since: builds[8].ID(), Limit: 2}))
			})
		})
	})

	Describe("Build", func() {
		var firstBuild dbng.Build

		Context("when a build exists", func() {
			BeforeEach(func() {
				var err error
				firstBuild, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())
			})

			It("finds the build", func() {
				build, found, err := job.Build(firstBuild.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.ID()).To(Equal(firstBuild.ID()))
				Expect(build.Status()).To(Equal(firstBuild.Status()))
			})
		})

		Context("when the build does not exist", func() {
			It("does not error", func() {
				build, found, err := job.Build("bogus-build")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(build).To(BeNil())
			})
		})
	})

	Describe("GetRunningBuildsBySerialGroup", func() {
		Describe("same job", func() {
			var startedBuild, scheduledBuild dbng.Build

			BeforeEach(func() {
				var err error
				_, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				startedBuild, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())
				_, err = startedBuild.Start("", "")
				Expect(err).NotTo(HaveOccurred())

				scheduledBuild, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				scheduled, err := scheduledBuild.Schedule()
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeTrue())

				for _, s := range []dbng.BuildStatus{dbng.BuildStatusSucceeded, dbng.BuildStatusFailed, dbng.BuildStatusErrored, dbng.BuildStatusAborted} {
					finishedBuild, err := job.CreateBuild()
					Expect(err).NotTo(HaveOccurred())

					scheduled, err = finishedBuild.Schedule()
					Expect(err).NotTo(HaveOccurred())
					Expect(scheduled).To(BeTrue())

					err = finishedBuild.Finish(s)
					Expect(err).NotTo(HaveOccurred())
				}

				otherJob, found, err := pipeline.Job("some-other-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				_, err = otherJob.CreateBuild()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a list of running or schedule builds for said job", func() {
				builds, err := job.GetRunningBuildsBySerialGroup([]string{"serial-group"})
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(2))
				ids := []int{}
				for _, build := range builds {
					ids = append(ids, build.ID())
				}
				Expect(ids).To(ConsistOf([]int{startedBuild.ID(), scheduledBuild.ID()}))
			})
		})

		Describe("multiple jobs with same serial group", func() {
			var serialGroupBuild dbng.Build

			BeforeEach(func() {
				var err error
				_, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				otherSerialJob, found, err := pipeline.Job("other-serial-group-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				serialGroupBuild, err = otherSerialJob.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				scheduled, err := serialGroupBuild.Schedule()
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeTrue())

				differentSerialJob, found, err := pipeline.Job("different-serial-group-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				differentSerialGroupBuild, err := differentSerialJob.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				scheduled, err = differentSerialGroupBuild.Schedule()
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeTrue())
			})

			It("returns a list of builds in the same serial group", func() {
				builds, err := job.GetRunningBuildsBySerialGroup([]string{"serial-group"})
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(1))
				Expect(builds[0].ID()).To(Equal(serialGroupBuild.ID()))
			})
		})
	})

	Describe("GetNextPendingBuildBySerialGroup", func() {
		var job1Name, job2Name string
		var job1, job2 dbng.Job

		BeforeEach(func() {
			job1Name = "some-job"
			job2Name = "other-serial-group-job"

			var found bool
			var err error
			job1, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			job2, found, err = pipeline.Job("other-serial-group-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when some jobs have builds with inputs determined as false", func() {
			var actualBuild dbng.Build

			BeforeEach(func() {
				_, err := job1.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				actualBuild, err = job2.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				err = job2.SaveNextInputMapping(nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the next most pending build in a group of jobs", func() {
				build, found, err := job1.GetNextPendingBuildBySerialGroup([]string{"serial-group"})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.ID()).To(Equal(actualBuild.ID()))
			})
		})

		It("should return the next most pending build in a group of jobs", func() {
			buildOne, err := job1.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			buildTwo, err := job1.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			buildThree, err := job2.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			err = job1.SaveNextInputMapping(nil)
			Expect(err).NotTo(HaveOccurred())
			err = job2.SaveNextInputMapping(nil)
			Expect(err).NotTo(HaveOccurred())

			build, found, err := job1.GetNextPendingBuildBySerialGroup([]string{"serial-group"})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.ID()).To(Equal(buildOne.ID()))

			build, found, err = job2.GetNextPendingBuildBySerialGroup([]string{"serial-group", "really-different-group"})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.ID()).To(Equal(buildOne.ID()))

			Expect(buildOne.Finish(dbng.BuildStatusSucceeded)).To(Succeed())

			build, found, err = job1.GetNextPendingBuildBySerialGroup([]string{"serial-group"})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.ID()).To(Equal(buildTwo.ID()))

			build, found, err = job2.GetNextPendingBuildBySerialGroup([]string{"serial-group", "really-different-group"})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.ID()).To(Equal(buildTwo.ID()))

			scheduled, err := buildTwo.Schedule()
			Expect(err).NotTo(HaveOccurred())
			Expect(scheduled).To(BeTrue())
			Expect(buildTwo.Finish(dbng.BuildStatusSucceeded)).To(Succeed())

			build, found, err = job1.GetNextPendingBuildBySerialGroup([]string{"serial-group"})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.ID()).To(Equal(buildThree.ID()))

			build, found, err = job2.GetNextPendingBuildBySerialGroup([]string{"serial-group", "really-different-group"})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.ID()).To(Equal(buildThree.ID()))
		})
	})

	Describe("NextBuildInputs", func() {
		var pipeline2 dbng.Pipeline
		var versions dbng.SavedVersionedResources
		var job dbng.Job
		var job2 dbng.Job

		BeforeEach(func() {
			resourceConfig := atc.ResourceConfig{
				Name: "some-resource",
				Type: "some-type",
			}

			err := pipeline.SaveResourceVersions(
				resourceConfig,
				[]atc.Version{
					{"version": "v1"},
					{"version": "v2"},
					{"version": "v3"},
				},
			)
			Expect(err).NotTo(HaveOccurred())

			var found bool
			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			// save metadata for v1
			build, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())
			err = build.SaveInput(dbng.BuildInput{
				Name: "some-input",
				VersionedResource: dbng.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version:  dbng.ResourceVersion{"version": "v1"},
					Metadata: []dbng.ResourceMetadataField{{Name: "name1", Value: "value1"}},
				},
				FirstOccurrence: true,
			})
			Expect(err).NotTo(HaveOccurred())

			reversions, _, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{Limit: 3})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			versions = []dbng.SavedVersionedResource{reversions[2], reversions[1], reversions[0]}

			config := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
					{
						Name: "some-other-job",
					},
				},
				Resources: atc.ResourceConfigs{resourceConfig},
			}

			pipeline2, _, err = team.SavePipeline("some-pipeline-2", config, 1, dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			job2, found, err = pipeline2.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

		})

		Describe("independent build inputs", func() {
			It("gets independent build inputs for the given job name", func() {
				inputVersions := algorithm.InputMapping{
					"some-input-1": algorithm.InputVersion{
						VersionID:       versions[0].ID,
						FirstOccurrence: false,
					},
					"some-input-2": algorithm.InputVersion{
						VersionID:       versions[1].ID,
						FirstOccurrence: true,
					},
				}
				err := job.SaveIndependentInputMapping(inputVersions)
				Expect(err).NotTo(HaveOccurred())

				pipeline2InputVersions := algorithm.InputMapping{
					"some-input-3": algorithm.InputVersion{
						VersionID:       versions[2].ID,
						FirstOccurrence: false,
					},
				}
				err = job2.SaveIndependentInputMapping(pipeline2InputVersions)
				Expect(err).NotTo(HaveOccurred())

				buildInputs := []dbng.BuildInput{
					{
						Name:              "some-input-1",
						VersionedResource: versions[0].VersionedResource,
						FirstOccurrence:   false,
					},
					{
						Name:              "some-input-2",
						VersionedResource: versions[1].VersionedResource,
						FirstOccurrence:   true,
					},
				}

				actualBuildInputs, err := job.GetIndependentBuildInputs()
				Expect(err).NotTo(HaveOccurred())

				Expect(actualBuildInputs).To(ConsistOf(buildInputs))

				By("updating the set of independent build inputs")
				inputVersions2 := algorithm.InputMapping{
					"some-input-2": algorithm.InputVersion{
						VersionID:       versions[2].ID,
						FirstOccurrence: false,
					},
					"some-input-3": algorithm.InputVersion{
						VersionID:       versions[2].ID,
						FirstOccurrence: true,
					},
				}
				err = job.SaveIndependentInputMapping(inputVersions2)
				Expect(err).NotTo(HaveOccurred())

				buildInputs2 := []dbng.BuildInput{
					{
						Name:              "some-input-2",
						VersionedResource: versions[2].VersionedResource,
						FirstOccurrence:   false,
					},
					{
						Name:              "some-input-3",
						VersionedResource: versions[2].VersionedResource,
						FirstOccurrence:   true,
					},
				}

				actualBuildInputs2, err := job.GetIndependentBuildInputs()
				Expect(err).NotTo(HaveOccurred())

				Expect(actualBuildInputs2).To(ConsistOf(buildInputs2))

				By("updating independent build inputs to an empty set when the mapping is nil")
				err = job.SaveIndependentInputMapping(nil)
				Expect(err).NotTo(HaveOccurred())

				actualBuildInputs3, err := job.GetIndependentBuildInputs()
				Expect(err).NotTo(HaveOccurred())
				Expect(actualBuildInputs3).To(BeEmpty())
			})
		})

		Describe("next build inputs", func() {
			It("gets next build inputs for the given job name", func() {
				inputVersions := algorithm.InputMapping{
					"some-input-1": algorithm.InputVersion{
						VersionID:       versions[0].ID,
						FirstOccurrence: false,
					},
					"some-input-2": algorithm.InputVersion{
						VersionID:       versions[1].ID,
						FirstOccurrence: true,
					},
				}
				err := job.SaveNextInputMapping(inputVersions)
				Expect(err).NotTo(HaveOccurred())

				pipeline2InputVersions := algorithm.InputMapping{
					"some-input-3": algorithm.InputVersion{
						VersionID:       versions[2].ID,
						FirstOccurrence: false,
					},
				}
				err = job2.SaveNextInputMapping(pipeline2InputVersions)
				Expect(err).NotTo(HaveOccurred())

				buildInputs := []dbng.BuildInput{
					{
						Name:              "some-input-1",
						VersionedResource: versions[0].VersionedResource,
						FirstOccurrence:   false,
					},
					{
						Name:              "some-input-2",
						VersionedResource: versions[1].VersionedResource,
						FirstOccurrence:   true,
					},
				}

				actualBuildInputs, found, err := job.GetNextBuildInputs()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(actualBuildInputs).To(ConsistOf(buildInputs))

				By("updating the set of next build inputs")
				inputVersions2 := algorithm.InputMapping{
					"some-input-2": algorithm.InputVersion{
						VersionID:       versions[2].ID,
						FirstOccurrence: false,
					},
					"some-input-3": algorithm.InputVersion{
						VersionID:       versions[2].ID,
						FirstOccurrence: true,
					},
				}
				err = job.SaveNextInputMapping(inputVersions2)
				Expect(err).NotTo(HaveOccurred())

				buildInputs2 := []dbng.BuildInput{
					{
						Name:              "some-input-2",
						VersionedResource: versions[2].VersionedResource,
						FirstOccurrence:   false,
					},
					{
						Name:              "some-input-3",
						VersionedResource: versions[2].VersionedResource,
						FirstOccurrence:   true,
					},
				}

				actualBuildInputs2, found, err := job.GetNextBuildInputs()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(actualBuildInputs2).To(ConsistOf(buildInputs2))

				By("updating next build inputs to an empty set when the mapping is nil")
				err = job.SaveNextInputMapping(nil)
				Expect(err).NotTo(HaveOccurred())

				actualBuildInputs3, found, err := job.GetNextBuildInputs()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(actualBuildInputs3).To(BeEmpty())
			})

			It("distinguishes between a job with no inputs and a job with missing inputs", func() {
				By("initially returning not found")
				_, found, err := job.GetNextBuildInputs()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())

				By("returning found when an empty input mapping is saved")
				err = job.SaveNextInputMapping(algorithm.InputMapping{})
				Expect(err).NotTo(HaveOccurred())

				_, found, err = job.GetNextBuildInputs()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				By("returning not found when the input mapping is deleted")
				err = job.DeleteNextInputMapping()
				Expect(err).NotTo(HaveOccurred())

				_, found, err = job.GetNextBuildInputs()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("saving build inputs", func() {
		var (
			buildMetadata []dbng.ResourceMetadataField
			vr1           dbng.VersionedResource
		)

		BeforeEach(func() {
			buildMetadata = []dbng.ResourceMetadataField{
				{
					Name:  "meta1",
					Value: "value1",
				},
				{
					Name:  "meta2",
					Value: "value2",
				},
			}

			vr1 = dbng.VersionedResource{
				Resource: "some-other-resource",
				Type:     "some-type",
				Version:  dbng.ResourceVersion{"ver": "2"},
			}
		})

		It("fails to save build input if resource does not exist", func() {
			build, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			vr := dbng.VersionedResource{
				Resource: "unknown-resource",
				Type:     "some-type",
				Version:  dbng.ResourceVersion{"ver": "2"},
			}

			input := dbng.BuildInput{
				Name:              "some-input",
				VersionedResource: vr,
			}

			err = build.SaveInput(input)
			Expect(err).To(HaveOccurred())
		})

		It("updates metadata of existing versioned resources", func() {
			build, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			err = build.SaveInput(dbng.BuildInput{
				Name:              "some-input",
				VersionedResource: vr1,
			})
			Expect(err).NotTo(HaveOccurred())

			inputs, _, err := build.Resources()
			Expect(err).NotTo(HaveOccurred())
			Expect(inputs).To(ConsistOf([]dbng.BuildInput{
				{Name: "some-input", VersionedResource: vr1, FirstOccurrence: true},
			}))

			withMetadata := vr1
			withMetadata.Metadata = buildMetadata

			err = build.SaveInput(dbng.BuildInput{
				Name:              "some-other-input",
				VersionedResource: withMetadata,
			})
			Expect(err).NotTo(HaveOccurred())

			inputs, _, err = build.Resources()
			Expect(err).NotTo(HaveOccurred())
			Expect(inputs).To(ConsistOf([]dbng.BuildInput{
				{Name: "some-input", VersionedResource: withMetadata, FirstOccurrence: true},
				{Name: "some-other-input", VersionedResource: withMetadata, FirstOccurrence: true},
			}))

			err = build.SaveInput(dbng.BuildInput{
				Name:              "some-input",
				VersionedResource: withMetadata,
			})
			Expect(err).NotTo(HaveOccurred())

			inputs, _, err = build.Resources()
			Expect(err).NotTo(HaveOccurred())
			Expect(inputs).To(ConsistOf([]dbng.BuildInput{
				{Name: "some-input", VersionedResource: withMetadata, FirstOccurrence: true},
				{Name: "some-other-input", VersionedResource: withMetadata, FirstOccurrence: true},
			}))

		})

		It("does not clobber metadata of existing versioned resources", func() {
			build, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			withMetadata := vr1
			withMetadata.Metadata = buildMetadata

			withoutMetadata := vr1
			withoutMetadata.Metadata = nil

			err = build.SaveInput(dbng.BuildInput{
				Name:              "some-input",
				VersionedResource: withMetadata,
			})
			Expect(err).NotTo(HaveOccurred())

			inputs, _, err := build.Resources()
			Expect(err).NotTo(HaveOccurred())
			Expect(inputs).To(ConsistOf([]dbng.BuildInput{
				{Name: "some-input", VersionedResource: withMetadata, FirstOccurrence: true},
			}))

			err = build.SaveInput(dbng.BuildInput{
				Name:              "some-other-input",
				VersionedResource: withoutMetadata,
			})
			Expect(err).NotTo(HaveOccurred())

			inputs, _, err = build.Resources()
			Expect(err).NotTo(HaveOccurred())
			Expect(inputs).To(ConsistOf([]dbng.BuildInput{
				{Name: "some-input", VersionedResource: withMetadata, FirstOccurrence: true},
				{Name: "some-other-input", VersionedResource: withMetadata, FirstOccurrence: true},
			}))
		})
	})

	Describe("a build is created for a job", func() {
		var (
			build1DB      dbng.Build
			otherPipeline dbng.Pipeline
			otherJob      dbng.Job
		)

		BeforeEach(func() {
			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name: "some-other-resource",
						Type: "some-type",
					},
				},
			}
			var err error
			otherPipeline, _, err = team.SavePipeline("some-other-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			build1DB, err = job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			Expect(build1DB.ID()).NotTo(BeZero())
			Expect(build1DB.JobName()).To(Equal("some-job"))
			Expect(build1DB.Name()).To(Equal("1"))
			Expect(build1DB.Status()).To(Equal(dbng.BuildStatusPending))
			Expect(build1DB.IsScheduled()).To(BeFalse())

			var found bool
			otherJob, found, err = otherPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		It("becomes the next pending build for job", func() {
			nextPendings, err := job.GetPendingBuilds()
			Expect(err).NotTo(HaveOccurred())
			//time.Sleep(10 * time.Hour)
			Expect(nextPendings).NotTo(BeEmpty())
			Expect(nextPendings[0].ID()).To(Equal(build1DB.ID()))
		})

		It("is in the list of pending builds", func() {
			nextPendingBuilds, err := pipeline.GetAllPendingBuilds()
			Expect(err).NotTo(HaveOccurred())
			Expect(nextPendingBuilds["some-job"]).To(HaveLen(1))
			Expect(nextPendingBuilds["some-job"]).To(Equal([]dbng.Build{build1DB}))
		})

		Context("and another build for a different pipeline is created with the same job name", func() {
			BeforeEach(func() {
				otherBuild, err := otherJob.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				Expect(otherBuild.ID()).NotTo(BeZero())
				Expect(otherBuild.JobName()).To(Equal("some-job"))
				Expect(otherBuild.Name()).To(Equal("1"))
				Expect(otherBuild.Status()).To(Equal(dbng.BuildStatusPending))
				Expect(otherBuild.IsScheduled()).To(BeFalse())
			})

			It("does not change the next pending build for job", func() {
				nextPendingBuilds, err := job.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(nextPendingBuilds).To(Equal([]dbng.Build{build1DB}))
			})

			It("does not change pending builds", func() {
				nextPendingBuilds, err := pipeline.GetAllPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(nextPendingBuilds["some-job"]).To(HaveLen(1))
				Expect(nextPendingBuilds["some-job"]).To(Equal([]dbng.Build{build1DB}))
			})
		})

		Context("when scheduled", func() {
			BeforeEach(func() {
				var err error
				var found bool
				found, err = build1DB.Schedule()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("remains the next pending build for job", func() {
				nextPendingBuilds, err := job.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(nextPendingBuilds).NotTo(BeEmpty())
				Expect(nextPendingBuilds[0].ID()).To(Equal(build1DB.ID()))
			})

			It("remains in the list of pending builds", func() {
				nextPendingBuilds, err := pipeline.GetAllPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(nextPendingBuilds["some-job"]).To(HaveLen(1))
				Expect(nextPendingBuilds["some-job"][0].ID()).To(Equal(build1DB.ID()))
			})
		})

		Context("when started", func() {
			BeforeEach(func() {
				started, err := build1DB.Start("some-engine", "some-metadata")
				Expect(err).NotTo(HaveOccurred())
				Expect(started).To(BeTrue())
			})

			It("saves the updated status, and the engine and engine metadata", func() {
				found, err := build1DB.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build1DB.Status()).To(Equal(dbng.BuildStatusStarted))
				Expect(build1DB.Engine()).To(Equal("some-engine"))
				Expect(build1DB.EngineMetadata()).To(Equal("some-metadata"))
			})

			It("saves the build's start time", func() {
				found, err := build1DB.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build1DB.StartTime().Unix()).To(BeNumerically("~", time.Now().Unix(), 3))
			})
		})

		Context("when the build finishes", func() {
			BeforeEach(func() {
				err := build1DB.Finish(dbng.BuildStatusSucceeded)
				Expect(err).NotTo(HaveOccurred())
			})

			It("sets the build's status and end time", func() {
				found, err := build1DB.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build1DB.Status()).To(Equal(dbng.BuildStatusSucceeded))
				Expect(build1DB.EndTime().Unix()).To(BeNumerically("~", time.Now().Unix(), 3))
			})
		})

		Context("and another is created for the same job", func() {
			var build2DB dbng.Build

			BeforeEach(func() {
				var err error
				build2DB, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				Expect(build2DB.ID()).NotTo(BeZero())
				Expect(build2DB.ID()).NotTo(Equal(build1DB.ID()))
				Expect(build2DB.Name()).To(Equal("2"))
				Expect(build2DB.Status()).To(Equal(dbng.BuildStatusPending))
			})

			Describe("the first build", func() {
				It("remains the next pending build", func() {
					nextPendingBuilds, err := job.GetPendingBuilds()
					Expect(err).NotTo(HaveOccurred())
					Expect(nextPendingBuilds).To(HaveLen(2))
					Expect(nextPendingBuilds[0].ID()).To(Equal(build1DB.ID()))
					Expect(nextPendingBuilds[1].ID()).To(Equal(build2DB.ID()))
				})

				It("remains in the list of pending builds", func() {
					nextPendingBuilds, err := pipeline.GetAllPendingBuilds()
					Expect(err).NotTo(HaveOccurred())
					Expect(nextPendingBuilds["some-job"]).To(HaveLen(2))
					Expect(nextPendingBuilds["some-job"]).To(ConsistOf(build1DB, build2DB))
				})
			})
		})
	})

	Describe("EnsurePendingBuildExists", func() {
		Context("when only a started build exists", func() {
			BeforeEach(func() {
				build1, err := job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				started, err := build1.Start("some-engine", "some-metadata")
				Expect(err).NotTo(HaveOccurred())
				Expect(started).To(BeTrue())
			})

			It("creates a build", func() {
				err := job.EnsurePendingBuildExists()
				Expect(err).NotTo(HaveOccurred())

				pendingBuilds, err := job.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(pendingBuilds).To(HaveLen(1))
			})

			It("doesn't create another build the second time it's called", func() {
				err := job.EnsurePendingBuildExists()
				Expect(err).NotTo(HaveOccurred())

				err = job.EnsurePendingBuildExists()
				Expect(err).NotTo(HaveOccurred())

				builds2, err := job.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(builds2).To(HaveLen(1))

				started, err := builds2[0].Start("some-engine", "some-metadata")
				Expect(err).NotTo(HaveOccurred())
				Expect(started).To(BeTrue())

				builds2, err = job.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(builds2).To(HaveLen(0))
			})
		})
	})
})
