package db_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PipelineLifecycle", func() {
	var (
		pl                db.PipelineLifecycle
		child1Pipeline    db.Pipeline
		child2Pipeline    db.Pipeline
		child3Pipeline    db.Pipeline
		pipelinesArchived int
		err               error
	)

	BeforeEach(func() {
		pl = db.NewPipelineLifecycle(dbConn, lockFactory)
	})

	JustBeforeEach(func() {
		pipelinesArchived, err = pl.ArchiveAbandonedPipelines()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("pipeline is no longer set by latest successful build of parent job", func() {
		BeforeEach(func() {
			By("creating three child pipelines")
			build, _ := defaultJob.CreateBuild()
			child1Pipeline, _, _ = build.SavePipeline("child1-pipeline", defaultTeam.ID(), defaultPipelineConfig, db.ConfigVersion(0), false)
			child2Pipeline, _, _ = build.SavePipeline("child2-pipeline", defaultTeam.ID(), defaultPipelineConfig, db.ConfigVersion(0), false)
			child3Pipeline, _, _ = build.SavePipeline("child3-pipeline", defaultTeam.ID(), defaultPipelineConfig, db.ConfigVersion(0), false)
			build.Finish(db.BuildStatusSucceeded)

			By("running a second build that sets 2/3 pipelines")
			build2, _ := defaultJob.CreateBuild()
			child1Pipeline, _, _ = build2.SavePipeline("child1-pipeline", defaultTeam.ID(), defaultPipelineConfig, child1Pipeline.ConfigVersion(), false)
			child2Pipeline, _, _ = build2.SavePipeline("child2-pipeline", defaultTeam.ID(), defaultPipelineConfig, child2Pipeline.ConfigVersion(), false)
			build2.Finish(db.BuildStatusSucceeded)
		})

		It("archives the child pipeline no longer being set", func() {
			Expect(pipelinesArchived).To(Equal(1))
			child3Pipeline.Reload()
			Expect(child3Pipeline.Archived()).To(BeTrue())
		})
	})

	Context("the build succeeds once then fails", func() {
		BeforeEach(func() {
			By("first build is successful")
			build, _ := defaultJob.CreateBuild()
			child1Pipeline, _, _ = build.SavePipeline("child1-pipeline", defaultTeam.ID(), defaultPipelineConfig, db.ConfigVersion(0), false)
			child2Pipeline, _, _ = build.SavePipeline("child2-pipeline", defaultTeam.ID(), defaultPipelineConfig, db.ConfigVersion(0), false)
			build.Finish(db.BuildStatusSucceeded)

			By("second build fails and sets no pipelines")
			build2, _ := defaultJob.CreateBuild()
			child1Pipeline, _, _ = build2.SavePipeline("child1-pipeline", defaultTeam.ID(), defaultPipelineConfig, child1Pipeline.ConfigVersion(), false)
			build2.Finish(db.BuildStatusFailed)
		})

		It("does not archive the pipelines", func() {
			Expect(pipelinesArchived).To(Equal(0))
		})
	})

	Context("child pipeline is set by a job in a pipeline", func() {
		BeforeEach(func() {
			build, _ := defaultJob.CreateBuild()
			child1Pipeline, _, _ = build.SavePipeline("child1-pipeline", defaultTeam.ID(), defaultPipelineConfig, db.ConfigVersion(0), false)
			build.Finish(db.BuildStatusSucceeded)
		})

		Context("parent pipeline is destroyed", func() {
			BeforeEach(func() {
				defaultPipeline.Destroy()
			})

			It("should archive all child pipelines", func() {
				Expect(pipelinesArchived).To(Equal(1))
				child1Pipeline.Reload()
				Expect(child1Pipeline.Archived()).To(BeTrue())
			})
		})

		Context("job is removed from the parent pipeline", func() {
			BeforeEach(func() {
				defaultPipelineConfig.Jobs = atc.JobConfigs{
					{
						Name: "a-different-job",
					},
				}
				defaultTeam.SavePipeline("default-pipeline", defaultPipelineConfig, defaultPipeline.ConfigVersion(), false)
			})

			It("archives all child pipelines set by the deleted job", func() {
				Expect(pipelinesArchived).To(Equal(1))
				child1Pipeline.Reload()
				Expect(child1Pipeline.Archived()).To(BeTrue())
			})
		})
	})

	Context("pipeline does not have a parent job and build ID", func() {
		It("Should not archive the pipeline", func() {
			Expect(pipelinesArchived).To(Equal(0))

			defaultPipeline.Reload()
			Expect(defaultPipeline.Archived()).To(BeFalse())
		})
	})
})
