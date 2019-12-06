package db_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Job Factory", func() {
	var jobFactory db.JobFactory

	BeforeEach(func() {
		jobFactory = db.NewJobFactory(dbConn, lockFactory)
	})

	Context("when there are public and private pipelines", func() {
		var publicPipeline db.Pipeline

		BeforeEach(func() {
			otherTeam, err := teamFactory.CreateTeam(atc.Team{Name: "other-team"})
			Expect(err).NotTo(HaveOccurred())

			publicPipeline, _, err = otherTeam.SavePipeline("public-pipeline", atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "public-pipeline-job"},
				},
			}, db.ConfigVersion(0), false)
			Expect(err).ToNot(HaveOccurred())
			Expect(publicPipeline.Expose()).To(Succeed())

			_, _, err = otherTeam.SavePipeline("private-pipeline", atc.Config{
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
				allJobs, err := jobFactory.AllActiveJobs()
				Expect(err).ToNot(HaveOccurred())

				Expect(len(allJobs)).To(Equal(3))
				Expect(allJobs[0].Job.Name()).To(Equal("some-job"))
				Expect(allJobs[1].Job.Name()).To(Equal("public-pipeline-job"))
				Expect(allJobs[2].Job.Name()).To(Equal("private-pipeline-job"))
			})
		})
	})

	Describe("JobsToSchedule", func() {
		var (
			job1 db.Job
			job2 db.Job
			job3 db.Job
		)

		BeforeEach(func() {
			err := defaultPipeline.Destroy()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the job has a requested schedule time later than the last scheduled", func() {
			BeforeEach(func() {
				pipeline1, _, err := defaultTeam.SavePipeline("fake-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-name"},
					},
				}, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())

				var found bool
				job1, found, err = pipeline1.Job("job-name")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = job1.RequestSchedule()
				Expect(err).ToNot(HaveOccurred())
			})

			It("fetches that pipeline", func() {
				jobs, err := jobFactory.JobsToSchedule()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(jobs)).To(Equal(1))
				Expect(jobs[0].Name()).To(Equal(job1.Name()))
			})
		})

		Context("when the job has a requested schedule time earlier than the last scheduled", func() {
			BeforeEach(func() {
				pipeline1, _, err := defaultTeam.SavePipeline("fake-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-name"},
					},
				}, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())

				var found bool
				job1, found, err = pipeline1.Job("job-name")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				_, err = dbConn.Exec("UPDATE jobs SET last_scheduled = now() WHERE id = $1;", job1.ID())
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not fetch that pipeline", func() {
				jobs, err := jobFactory.JobsToSchedule()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(jobs)).To(Equal(0))
			})
		})

		Context("when the job has a requested schedule time is the same as the last scheduled", func() {
			BeforeEach(func() {
				pipeline1, _, err := defaultTeam.SavePipeline("fake-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-name"},
					},
				}, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())

				var found bool
				job1, found, err = pipeline1.Job("job-name")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = job1.RequestSchedule()
				Expect(err).ToNot(HaveOccurred())

				found, err = job1.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = job1.UpdateLastScheduled(job1.ScheduleRequestedTime())
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not fetch that pipeline", func() {
				jobs, err := jobFactory.JobsToSchedule()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(jobs)).To(Equal(0))
			})
		})

		Context("when there are multiple pipelines with different times", func() {
			BeforeEach(func() {
				pipeline1, _, err := defaultTeam.SavePipeline("fake-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-name"},
					},
				}, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())

				var found bool
				job1, found, err = pipeline1.Job("job-name")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = job1.RequestSchedule()
				Expect(err).ToNot(HaveOccurred())

				team, err := teamFactory.CreateTeam(atc.Team{Name: "some-team"})
				Expect(err).ToNot(HaveOccurred())

				pipeline2, _, err := team.SavePipeline("fake-pipeline-two", atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-fake"},
					},
				}, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())

				job2, found, err = pipeline2.Job("job-fake")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				pipeline3, _, err := team.SavePipeline("fake-pipeline-three", atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-fake-two"},
					},
				}, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())

				job3, found, err = pipeline3.Job("job-fake-two")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				_, err = dbConn.Exec("UPDATE jobs SET last_scheduled = now() WHERE id = $1;", job2.ID())
				Expect(err).ToNot(HaveOccurred())

				err = job3.RequestSchedule()
				Expect(err).ToNot(HaveOccurred())
			})

			It("fetches the pipelines that have a requested schedule time later than it's last scheduled", func() {
				jobs, err := jobFactory.JobsToSchedule()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(jobs)).To(Equal(2))
				jobNames := []string{jobs[0].Name(), jobs[1].Name()}
				Expect(jobNames).To(ConsistOf(job1.Name(), job3.Name()))
			})
		})
	})
})
