package db_test

import (
	"context"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PipelinePauser", func() {
	var (
		pauser               db.PipelinePauser
		twoJobPipeline       db.Pipeline
		err                  error
		twoJobPipelineConfig = atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "job-one",
				},
				{
					Name: "job-two",
				},
			},
		}
		pipelineRef = atc.PipelineRef{Name: "twojobs-pipeline"}
	)

	BeforeEach(func() {
		pauser = db.NewPipelinePauser(dbConn, lockFactory)
	})

	Describe("PausePipelines that haven't run in more than 10 days", func() {
		Context("last run was 15 days ago", func() {
			Context("and one job has zero builds", func() {
				It("should be paused", func() {
					By("creating a pipeline with two jobs")
					twoJobPipeline, _, err = defaultTeam.SavePipeline(pipelineRef, twoJobPipelineConfig, db.ConfigVersion(0), false)
					Expect(err).NotTo(HaveOccurred())
					Expect(twoJobPipeline.Paused()).To(BeFalse(), "pipeline should start unpaused")
					By("making it look like the pipeline was set 15 days ago as well")
					_, err = dbConn.Exec(`UPDATE pipelines SET last_updated = NOW() - INTERVAL '15' DAY WHERE id = $1`, twoJobPipeline.ID())
					Expect(err).NotTo(HaveOccurred())

					By("creating a job that ran 15 days ago")
					jobOne, found, err := twoJobPipeline.Job("job-one")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					b1, err := jobOne.CreateBuild(defaultBuildCreatedBy)
					Expect(err).NotTo(HaveOccurred())
					b1.Finish(db.BuildStatusSucceeded)
					_, err = dbConn.Exec(`UPDATE builds SET end_time = NOW() - INTERVAL '15' DAY WHERE id = $1`, b1.ID())
					Expect(err).NotTo(HaveOccurred())

					By("having a second job with zero builds")
					// yup, do nothing

					By("running the pipeline pauser")
					err = pauser.PausePipelines(context.TODO(), 10)
					Expect(err).NotTo(HaveOccurred())

					_, err = twoJobPipeline.Reload()
					Expect(err).To(BeNil())
					Expect(twoJobPipeline.Paused()).To(BeTrue(), "pipeline should be paused")
				})
			})

			Context("all jobs have builds", func() {
				It("should be paused", func() {
					By("creating a pipeline with two jobs")
					twoJobPipeline, _, err = defaultTeam.SavePipeline(pipelineRef, twoJobPipelineConfig, db.ConfigVersion(0), false)
					Expect(err).NotTo(HaveOccurred())
					Expect(twoJobPipeline.Paused()).To(BeFalse(), "pipeline should start unpaused")
					By("making it look like the pipeline was set 15 days ago as well")
					_, err = dbConn.Exec(`UPDATE pipelines SET last_updated = NOW() - INTERVAL '15' DAY WHERE id = $1`, twoJobPipeline.ID())
					Expect(err).NotTo(HaveOccurred())

					By("creating a job that ran 15 days ago")
					jobOne, found, err := twoJobPipeline.Job("job-one")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					b1, err := jobOne.CreateBuild(defaultBuildCreatedBy)
					Expect(err).NotTo(HaveOccurred())
					b1.Finish(db.BuildStatusSucceeded)
					_, err = dbConn.Exec(`UPDATE builds SET end_time = NOW() - INTERVAL '15' DAY WHERE id = $1`, b1.ID())
					Expect(err).NotTo(HaveOccurred())

					By("creating a job that ran 20 days ago")
					jobTwo, found, err := twoJobPipeline.Job("job-two")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					b2, err := jobTwo.CreateBuild(defaultBuildCreatedBy)
					Expect(err).NotTo(HaveOccurred())
					b2.Finish(db.BuildStatusSucceeded)
					_, err = dbConn.Exec(`UPDATE builds SET end_time = NOW() - INTERVAL '20' DAY WHERE id = $1`, b2.ID())
					Expect(err).NotTo(HaveOccurred())

					By("running the pipeline pauser")
					err = pauser.PausePipelines(context.TODO(), 10)
					Expect(err).NotTo(HaveOccurred())

					_, err = twoJobPipeline.Reload()
					Expect(err).To(BeNil())
					Expect(twoJobPipeline.Paused()).To(BeTrue(), "pipeline should be paused")
				})
			})

			It("should say the pipeline was paused by 'automatic-pipeline-pauser'", func() {
				By("using the default pipeline with one job")
				Expect(defaultPipeline.Paused()).To(BeFalse(), "pipeline should start unpaused")
				By("making it look like the pipeline was set 20 days ago as well")
				_, err = dbConn.Exec(`UPDATE pipelines SET last_updated = NOW() - INTERVAL '20' DAY WHERE id = $1`, defaultPipeline.ID())
				Expect(err).NotTo(HaveOccurred())

				By("creating a job that ran 20 days ago")
				b1, err := defaultJob.CreateBuild(defaultBuildCreatedBy)
				Expect(err).NotTo(HaveOccurred())
				b1.Finish(db.BuildStatusSucceeded)
				_, err = dbConn.Exec(`UPDATE builds SET end_time = NOW() - INTERVAL '20' DAY WHERE id = $1`, b1.ID())
				Expect(err).NotTo(HaveOccurred())

				By("running the pipeline pauser")
				err = pauser.PausePipelines(context.TODO(), 10)
				Expect(err).NotTo(HaveOccurred())

				_, err = defaultPipeline.Reload()
				Expect(err).To(BeNil())
				Expect(defaultPipeline.Paused()).To(BeTrue(), "pipeline should be paused")
				Expect(defaultPipeline.PausedBy()).To(Equal("automatic-pipeline-pauser"))
			})
		})
		Context("last run was 1 day ago", func() {
			It("should not be paused", func() {
				By("creating a pipeline with two jobs")
				twoJobPipeline, _, err = defaultTeam.SavePipeline(pipelineRef, twoJobPipelineConfig, db.ConfigVersion(0), false)
				Expect(err).NotTo(HaveOccurred())
				Expect(twoJobPipeline.Paused()).To(BeFalse(), "pipeline should start unpaused")

				By("creating a job that ran yesterday")
				jobOne, found, err := twoJobPipeline.Job("job-one")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				b1, err := jobOne.CreateBuild(defaultBuildCreatedBy)
				Expect(err).NotTo(HaveOccurred())
				b1.Finish(db.BuildStatusSucceeded)
				_, err = dbConn.Exec(`UPDATE builds SET end_time = NOW() - INTERVAL '1' DAY WHERE id = $1`, b1.ID())
				Expect(err).NotTo(HaveOccurred())

				By("creating a job that ran 11 days ago")
				jobTwo, found, err := twoJobPipeline.Job("job-two")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				jobTwo.CreateBuild(defaultBuildCreatedBy)
				b2, err := jobTwo.CreateBuild(defaultBuildCreatedBy)
				Expect(err).NotTo(HaveOccurred())
				b2.Finish(db.BuildStatusSucceeded)
				_, err = dbConn.Exec(`UPDATE builds SET end_time = NOW() - INTERVAL '11' DAY WHERE id = $1`, b2.ID())
				Expect(err).NotTo(HaveOccurred())

				By("running the pipeline pauser")
				err = pauser.PausePipelines(context.TODO(), 10)
				Expect(err).NotTo(HaveOccurred())

				_, err = twoJobPipeline.Reload()
				Expect(err).To(BeNil())
				Expect(twoJobPipeline.Paused()).To(BeFalse(), "pipeline should NOT be paused")
			})
		})
		Context("last run was 10 days ago", func() {
			It("should not be paused", func() {
				By("creating a pipeline with two jobs")
				twoJobPipeline, _, err = defaultTeam.SavePipeline(pipelineRef, twoJobPipelineConfig, db.ConfigVersion(0), false)
				Expect(err).NotTo(HaveOccurred())
				Expect(twoJobPipeline.Paused()).To(BeFalse(), "pipeline should start unpaused")

				By("creating a job that ran 10 days ago")
				jobOne, found, err := twoJobPipeline.Job("job-one")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				b1, err := jobOne.CreateBuild(defaultBuildCreatedBy)
				Expect(err).NotTo(HaveOccurred())
				b1.Finish(db.BuildStatusSucceeded)
				_, err = dbConn.Exec(`UPDATE builds SET end_time = NOW() - INTERVAL '10' DAY WHERE id = $1`, b1.ID())
				Expect(err).NotTo(HaveOccurred())

				By("creating a job that ran 20 days ago")
				jobTwo, found, err := twoJobPipeline.Job("job-two")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				jobTwo.CreateBuild(defaultBuildCreatedBy)
				b2, err := jobTwo.CreateBuild(defaultBuildCreatedBy)
				Expect(err).NotTo(HaveOccurred())
				b2.Finish(db.BuildStatusSucceeded)
				_, err = dbConn.Exec(`UPDATE builds SET end_time = NOW() - INTERVAL '20' DAY WHERE id = $1`, b2.ID())
				Expect(err).NotTo(HaveOccurred())

				By("running the pipeline pauser")
				err = pauser.PausePipelines(context.TODO(), 10)
				Expect(err).NotTo(HaveOccurred())

				_, err = twoJobPipeline.Reload()
				Expect(err).To(BeNil())
				Expect(twoJobPipeline.Paused()).To(BeFalse(), "pipeline should NOT be paused")
			})
		})
	})
	Describe("newly set pipeline whose jobs have no builds", func() {
		It("should not be paused if all of its jobs have no builds", func() {
			By("creating a new pipeline")
			newPipeline, _, err := defaultTeam.SavePipeline(atc.PipelineRef{Name: "new-pipeline"}, defaultPipelineConfig, db.ConfigVersion(0), false)
			Expect(err).NotTo(HaveOccurred())
			Expect(newPipeline.Paused()).To(BeFalse(), "pipeline should start unpaused")

			By("running the pipeline pauser")
			err = pauser.PausePipelines(context.TODO(), 10)
			Expect(err).NotTo(HaveOccurred())

			_, err = newPipeline.Reload()
			Expect(err).To(BeNil())
			Expect(newPipeline.Paused()).To(BeFalse(), "pipeline should NOT be paused")
		})
	})
	Describe("pipeline with a build currently running", func() {
		It("should not be paused", func() {
			By("using the default pipeline with one job")
			Expect(defaultPipeline.Paused()).To(BeFalse(), "pipeline should start unpaused")
			By("making it look like the pipeline was updated 5 days ago")
			_, err = dbConn.Exec(`UPDATE pipelines SET last_updated = NOW() - INTERVAL '5' DAY WHERE id = $1`, defaultPipeline.ID())
			Expect(err).NotTo(HaveOccurred())

			By("creating a build that ran 11 days ago")
			b1, err := defaultJob.CreateBuild(defaultBuildCreatedBy)
			Expect(err).NotTo(HaveOccurred())
			b1.Finish(db.BuildStatusSucceeded)
			_, err = dbConn.Exec(`UPDATE builds SET end_time = NOW() - INTERVAL '11' DAY WHERE id = $1`, b1.ID())
			Expect(err).NotTo(HaveOccurred())

			By("creating a build that's currently running")
			b2, err := defaultJob.CreateBuild(defaultBuildCreatedBy)
			Expect(err).NotTo(HaveOccurred())
			b2.Reload()
			Expect(b2.IsRunning()).To(Equal(true))

			By("running the pipeline pauser")
			err = pauser.PausePipelines(context.TODO(), 10)
			Expect(err).NotTo(HaveOccurred())

			_, err = defaultPipeline.Reload()
			Expect(err).To(BeNil())
			Expect(defaultPipeline.Paused()).To(BeFalse(), "pipeline should not be paused")
		})
	})
})
