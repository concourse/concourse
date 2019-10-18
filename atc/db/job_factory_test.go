package db_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Job Factory", func() {
	var jobFactory db.JobFactory
	var publicPipeline, privatePipeline db.Pipeline

	BeforeEach(func() {
		jobFactory = db.NewJobFactory(dbConn, lockFactory)

		otherTeam, err := teamFactory.CreateTeam(atc.Team{Name: "other-team"})
		Expect(err).NotTo(HaveOccurred())

		publicPipeline, _, err = otherTeam.SavePipeline("public-pipeline", atc.Config{
			Jobs: atc.JobConfigs{
				{Name: "public-pipeline-job"},
			},
		}, db.ConfigVersion(0), false)
		Expect(err).ToNot(HaveOccurred())
		Expect(publicPipeline.Expose()).To(Succeed())

		privatePipeline, _, err = otherTeam.SavePipeline("private-pipeline", atc.Config{
			Jobs: atc.JobConfigs{
				{Name: "private-pipeline-job"},
			},
		}, db.ConfigVersion(0), false)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("VisibleJobs", func() {
		It("returns jobs in the provided teams and jobs in public pipelines", func() {
			visibleJobs, err := jobFactory.VisibleJobs([]string{"default-team"})
			Expect(err).ToNot(HaveOccurred())

			Expect(len(visibleJobs)).To(Equal(2))
			Expect(visibleJobs[0].Job.Name()).To(Equal("some-job"))
			Expect(visibleJobs[1].Job.Name()).To(Equal("public-pipeline-job"))
		})

		It("returns next build, latest completed build, and transition build for each job", func() {
			job, found, err := defaultPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			transitionBuild, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = transitionBuild.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			finishedBuild, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = finishedBuild.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			found, err = finishedBuild.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			nextBuild, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			visibleJobs, err := jobFactory.VisibleJobs([]string{"default-team"})
			Expect(err).ToNot(HaveOccurred())

			Expect(visibleJobs[0].Job.Name()).To(Equal("some-job"))
			Expect(visibleJobs[0].NextBuild.ID()).To(Equal(nextBuild.ID()))
			Expect(visibleJobs[0].FinishedBuild.ID()).To(Equal(finishedBuild.ID()))
			Expect(visibleJobs[0].TransitionBuild.ID()).To(Equal(transitionBuild.ID()))
		})
	})

	Describe("AllActiveJobsForDashboard", func() {
		It("return all private and public pipelines", func() {
			allJobs, err := jobFactory.AllActiveJobsForDashboard()
			Expect(err).ToNot(HaveOccurred())

			Expect(len(allJobs)).To(Equal(3))
			Expect(allJobs[0].Job.Name()).To(Equal("some-job"))
			Expect(allJobs[1].Job.Name()).To(Equal("public-pipeline-job"))
			Expect(allJobs[2].Job.Name()).To(Equal("private-pipeline-job"))
		})
	})

	Describe("JobsForPipelines", func() {
		var otherTeamPipeline db.Pipeline

		BeforeEach(func() {
			otherTeam, err := teamFactory.CreateTeam(atc.Team{Name: "team-2"})
			Expect(err).NotTo(HaveOccurred())

			otherTeamPipeline, _, err = otherTeam.SavePipeline("other-team-pipeline", atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "some-job"},
					{Name: "some-other-job"},
					{Name: "inactive-job"},
				},
			}, db.ConfigVersion(0), false)
			Expect(err).ToNot(HaveOccurred())

			_, err = dbConn.Exec(`UPDATE jobs SET active = false WHERE name = 'inactive-job'`)
			Expect(err).NotTo(HaveOccurred())

			pausedPipeline, _, err := otherTeam.SavePipeline("paused-pipeline", atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-from-paused-pipeline"},
				},
			}, db.ConfigVersion(0), false)
			Expect(err).ToNot(HaveOccurred())

			err = pausedPipeline.Pause()
			Expect(err).ToNot(HaveOccurred())
		})

		It("return all active jobs within the db", func() {
			actualJobs, err := jobFactory.JobsForPipelines()
			Expect(err).ToNot(HaveOccurred())

			job1, found, err := otherTeamPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			job2, found, err := otherTeamPipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			job3, found, err := defaultPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			job4, found, err := publicPipeline.Job("public-pipeline-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			job5, found, err := privatePipeline.Job("private-pipeline-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			expectedJobs := db.PipelineJobs{
				defaultPipeline.ID():   db.Jobs{job3},
				publicPipeline.ID():    db.Jobs{job4},
				privatePipeline.ID():   db.Jobs{job5},
				otherTeamPipeline.ID(): db.Jobs{job1, job2},
			}

			Expect(actualJobs).To(HaveLen(4))
			Expect(actualJobs).To(Equal(expectedJobs))
		})
	})
})
