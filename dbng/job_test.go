package dbng_test

import (
	"github.com/concourse/atc"
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
								RootFsUri: "some-image",
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
		})

		It("can report a job's latest running and finished builds", func() {
			finished, next, err := job.FinishedAndNextBuild()
			Expect(err).NotTo(HaveOccurred())

			Expect(next).To(BeNil())
			Expect(finished).To(BeNil())

			finishedBuild, err := pipeline.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			err = finishedBuild.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			otherFinishedBuild, err := otherPipeline.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			err = otherFinishedBuild.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			finished, next, err = job.FinishedAndNextBuild()
			Expect(err).NotTo(HaveOccurred())

			Expect(next).To(BeNil())
			Expect(finished.ID()).To(Equal(finishedBuild.ID()))

			nextBuild, err := pipeline.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			started, err := nextBuild.Start("some-engine", "meta")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			otherNextBuild, err := otherPipeline.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			otherStarted, err := otherNextBuild.Start("some-engine", "meta")
			Expect(err).NotTo(HaveOccurred())
			Expect(otherStarted).To(BeTrue())

			finished, next, err = job.FinishedAndNextBuild()
			Expect(err).NotTo(HaveOccurred())

			Expect(next.ID()).To(Equal(nextBuild.ID()))
			Expect(finished.ID()).To(Equal(finishedBuild.ID()))

			anotherRunningBuild, err := pipeline.CreateJobBuild("some-job")
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
				build, err := pipeline.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				_, err = pipeline.CreateJobBuild("some-other-job")
				Expect(err).NotTo(HaveOccurred())

				builds[i] = build

				var found bool
				someJob, found, err = pipeline.Job("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				someOtherJob, found, err = pipeline.Job("some-other-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
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
				firstBuild, err = pipeline.CreateJobBuild(job.Name())
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
				_, err = pipeline.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				startedBuild, err = pipeline.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				_, err = startedBuild.Start("", "")
				Expect(err).NotTo(HaveOccurred())

				scheduledBuild, err = pipeline.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				scheduled, err := scheduledBuild.Schedule()
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeTrue())

				for _, s := range []dbng.BuildStatus{dbng.BuildStatusSucceeded, dbng.BuildStatusFailed, dbng.BuildStatusErrored, dbng.BuildStatusAborted} {
					finishedBuild, err := pipeline.CreateJobBuild("some-job")
					Expect(err).NotTo(HaveOccurred())

					scheduled, err = finishedBuild.Schedule()
					Expect(err).NotTo(HaveOccurred())
					Expect(scheduled).To(BeTrue())

					err = finishedBuild.Finish(s)
					Expect(err).NotTo(HaveOccurred())
				}

				_, err = pipeline.CreateJobBuild("some-other-job")
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
				_, err = pipeline.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				serialGroupBuild, err = pipeline.CreateJobBuild("other-serial-group-job")
				Expect(err).NotTo(HaveOccurred())

				scheduled, err := serialGroupBuild.Schedule()
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeTrue())

				differentSerialGroupBuild, err := pipeline.CreateJobBuild("different-serial-group-job")
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
				_, err := pipeline.CreateJobBuild(job1Name)
				Expect(err).NotTo(HaveOccurred())

				actualBuild, err = pipeline.CreateJobBuild(job2Name)
				Expect(err).NotTo(HaveOccurred())

				err = pipeline.SaveNextInputMapping(nil, "other-serial-group-job")
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
			buildOne, err := pipeline.CreateJobBuild(job1Name)
			Expect(err).NotTo(HaveOccurred())

			buildTwo, err := pipeline.CreateJobBuild(job1Name)
			Expect(err).NotTo(HaveOccurred())

			buildThree, err := pipeline.CreateJobBuild(job2Name)
			Expect(err).NotTo(HaveOccurred())

			err = pipeline.SaveNextInputMapping(nil, "some-job")
			Expect(err).NotTo(HaveOccurred())
			err = pipeline.SaveNextInputMapping(nil, "other-serial-group-job")
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
})
