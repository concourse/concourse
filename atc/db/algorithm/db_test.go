package algorithm_test

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/algorithm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DB tests", func() {
	Describe("OrderPassedJobs", func() {
		var (
			versionsDB  *algorithm.VersionsDB
			orderedJobs []int
			currentJob  int
			passedJobs  algorithm.JobSet
			setup       setupDB
		)

		BeforeEach(func() {
			setup = setupDB{
				teamID:      1,
				pipelineID:  1,
				psql:        sq.StatementBuilder.PlaceholderFormat(sq.Dollar).RunWith(dbConn),
				jobIDs:      StringMapping{},
				resourceIDs: StringMapping{},
				versionIDs:  StringMapping{},
			}

			setup.insertTeamsPipelines()

			setup.insertJob("current-job")
			currentJob = setup.jobIDs.ID("current-job")
		})

		JustBeforeEach(func() {
			versionsDB = &algorithm.VersionsDB{
				Runner:      dbConn,
				JobIDs:      setup.jobIDs,
				ResourceIDs: setup.resourceIDs,
			}

			var err error
			orderedJobs, err = versionsDB.OrderPassedJobs(currentJob, passedJobs)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there is a build for the current job", func() {
			BeforeEach(func() {
				setup.insertRowBuild(DBRow{
					Job:     "current-job",
					BuildID: 1,
				})
			})

			Context("when all the passed jobs have build pipes", func() {
				var passedJob1, passedJob2 int

				BeforeEach(func() {
					setup.insertRowBuild(DBRow{Job: "passed-job-2", BuildID: 2})
					setup.insertRowBuild(DBRow{Job: "passed-job-1", BuildID: 3})

					setup.insertBuildPipe(DBRow{FromBuildID: 2, ToBuildID: 1})
					setup.insertBuildPipe(DBRow{FromBuildID: 3, ToBuildID: 1})

					passedJob1 = setup.jobIDs.ID("passed-job-1")
					passedJob2 = setup.jobIDs.ID("passed-job-2")

					passedJobs = algorithm.JobSet{passedJob1: {}, passedJob2: {}}
				})

				Context("when some passed jobs have the same number of builds", func() {
					It("should order by job id", func() {
						Expect(orderedJobs).To(Equal([]int{passedJob2, passedJob1}))
					})
				})

				Context("when the passed jobs have different number of builds", func() {
					BeforeEach(func() {
						setup.insertRowBuild(DBRow{Job: "passed-job-2", BuildID: 4})
					})

					It("should be ordered by build numbers", func() {
						Expect(orderedJobs).To(Equal([]int{passedJob1, passedJob2}))
					})
				})
			})

			Context("when some of the passed jobs have build pipes", func() {
				var passedJob1, passedJob2, passedJob3, passedJob4, passedJob5 int

				BeforeEach(func() {
					setup.insertRowBuild(DBRow{Job: "passed-job-1", BuildID: 2})
					setup.insertRowBuild(DBRow{Job: "passed-job-1", BuildID: 3})
					setup.insertRowBuild(DBRow{Job: "passed-job-2", BuildID: 4})
					setup.insertJob("passed-job-3")
					setup.insertRowBuild(DBRow{Job: "passed-job-4", BuildID: 5})
					setup.insertRowBuild(DBRow{Job: "passed-job-5", BuildID: 6})
					setup.insertRowBuild(DBRow{Job: "passed-job-2", BuildID: 7})
					setup.insertRowBuild(DBRow{Job: "passed-job-2", BuildID: 8})

					setup.insertBuildPipe(DBRow{FromBuildID: 4, ToBuildID: 1})
					setup.insertBuildPipe(DBRow{FromBuildID: 6, ToBuildID: 1})

					passedJob1 = setup.jobIDs.ID("passed-job-1")
					passedJob2 = setup.jobIDs.ID("passed-job-2")
					passedJob3 = setup.jobIDs.ID("passed-job-3")
					passedJob4 = setup.jobIDs.ID("passed-job-4")
					passedJob5 = setup.jobIDs.ID("passed-job-5")

					passedJobs = algorithm.JobSet{passedJob1: {}, passedJob2: {}, passedJob3: {}, passedJob4: {}, passedJob5: {}}
				})

				It("should be ordered first by passed jobs that have build pipes and then by build numbers", func() {
					Expect(orderedJobs).To(Equal([]int{passedJob5, passedJob2, passedJob3, passedJob4, passedJob1}))
				})
			})

			Context("when none of the passed jobs have build pipes", func() {
				var passedJob1, passedJob2 int

				BeforeEach(func() {
					setup.insertRowBuild(DBRow{Job: "passed-job-1", BuildID: 2})
					setup.insertRowBuild(DBRow{Job: "passed-job-1", BuildID: 3})
					setup.insertRowBuild(DBRow{Job: "passed-job-2", BuildID: 4})

					passedJob1 = setup.jobIDs.ID("passed-job-1")
					passedJob2 = setup.jobIDs.ID("passed-job-2")

					passedJobs = algorithm.JobSet{passedJob1: {}, passedJob2: {}}
				})

				It("should be ordered by build numbers", func() {
					Expect(orderedJobs).To(Equal([]int{passedJob2, passedJob1}))
				})
			})
		})

		Context("when the current job has no builds", func() {
			var passedJob1, passedJob2 int

			BeforeEach(func() {
				setup.insertRowBuild(DBRow{Job: "passed-job-1", BuildID: 1})
				setup.insertRowBuild(DBRow{Job: "passed-job-1", BuildID: 2})
				setup.insertRowBuild(DBRow{Job: "passed-job-2", BuildID: 3})

				passedJob1 = setup.jobIDs.ID("passed-job-1")
				passedJob2 = setup.jobIDs.ID("passed-job-2")

				passedJobs = algorithm.JobSet{passedJob1: {}, passedJob2: {}}
			})

			It("should be ordered by build numbers", func() {
				Expect(orderedJobs).To(Equal([]int{passedJob2, passedJob1}))
			})
		})
	})
})
