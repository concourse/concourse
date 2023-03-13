package db_test

import (
	"fmt"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PipelineLifecycle", func() {
	var (
		pl  db.PipelineLifecycle
		err error
	)

	BeforeEach(func() {
		pl = db.NewPipelineLifecycle(dbConn, lockFactory)
	})

	Describe("ArchiveAbandonedPipelines", func() {
		JustBeforeEach(func() {
			err = pl.ArchiveAbandonedPipelines()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("child pipeline is set by a job in a pipeline", func() {
			var (
				childPipeline db.Pipeline
				setChildBuild db.Build
			)

			BeforeEach(func() {
				setChildBuild, _ = defaultJob.CreateBuild(defaultBuildCreatedBy)
				childPipeline, _, _ = setChildBuild.SavePipeline(atc.PipelineRef{Name: "child-pipeline"}, defaultTeam.ID(), defaultPipelineConfig, db.ConfigVersion(0), false)
				setChildBuild.Finish(db.BuildStatusSucceeded)
			})

			Context("parent and child pipelines happily exist", func() {
				It("neither pipeline should be archived", func() {
					childPipeline.Reload()
					defaultPipeline.Reload()
					Expect(childPipeline.Archived()).To(BeFalse())
					Expect(defaultPipeline.Archived()).To(BeFalse())
				})
			})

			Context("parent pipeline is destroyed", func() {
				BeforeEach(func() {
					err := defaultPipeline.Destroy()
					Expect(err).ToNot(HaveOccurred())
				})

				It("should archive all child pipelines", func() {
					childPipeline.Reload()
					Expect(childPipeline.Archived()).To(BeTrue())
				})
			})

			Context("needs to be archived but is already archived", func() {
				var lastUpdated time.Time

				BeforeEach(func() {
					err := childPipeline.Archive()
					Expect(err).ToNot(HaveOccurred())

					reloaded, err := childPipeline.Reload()
					Expect(err).ToNot(HaveOccurred())
					Expect(reloaded).To(BeTrue())

					lastUpdated = childPipeline.LastUpdated()

					err = defaultPipeline.Destroy()
					Expect(err).ToNot(HaveOccurred())
				})

				It("should not rearchive pipeline if it is already archived", func() {
					reloaded, err := childPipeline.Reload()
					Expect(err).ToNot(HaveOccurred())
					Expect(reloaded).To(BeTrue())

					Expect(childPipeline.Archived()).To(BeTrue())
					Expect(childPipeline.LastUpdated()).To(Equal(lastUpdated))
				})
			})

			Context("parent pipeline is archived", func() {
				BeforeEach(func() {
					err := defaultPipeline.Archive()
					Expect(err).ToNot(HaveOccurred())
				})

				It("should archive all child pipelines", func() {
					childPipeline.Reload()
					Expect(childPipeline.Archived()).To(BeTrue())
				})
			})

			Context("job is removed from the parent pipeline", func() {
				BeforeEach(func() {
					defaultPipelineConfig.Jobs = atc.JobConfigs{
						{
							Name: "a-different-job",
						},
					}
					defaultTeam.SavePipeline(defaultPipelineRef, defaultPipelineConfig, defaultPipeline.ConfigVersion(), false)
				})

				It("archives all child pipelines set by the deleted job", func() {
					childPipeline.Reload()
					Expect(childPipeline.Archived()).To(BeTrue())
				})
			})

			Context("when build that set child pipeline is later than latest completed build", func() {
				BeforeEach(func() {
					err := childPipeline.SetParentIDs(defaultJob.ID(), setChildBuild.ID()+1)
					Expect(err).ToNot(HaveOccurred())
				})

				It("should not archive child pipeline", func() {
					childPipeline.Reload()
					Expect(childPipeline.Archived()).To(BeFalse())
				})
			})

			Context("when build that set child pipeline is not most recent", func() {
				BeforeEach(func() {
					setChildBuild, _ = defaultJob.CreateBuild(defaultBuildCreatedBy)
					setChildBuild.Finish(db.BuildStatusSucceeded)
				})

				It("should archive child pipeline", func() {
					childPipeline.Reload()
					Expect(childPipeline.Archived()).To(BeTrue())
				})
			})

			Context("when build that set child pipeline is not most recent but was not successful", func() {
				BeforeEach(func() {
					setChildBuild, _ = defaultJob.CreateBuild(defaultBuildCreatedBy)
					setChildBuild.Finish(db.BuildStatusFailed)
				})

				It("should not archive child pipeline", func() {
					childPipeline.Reload()
					Expect(childPipeline.Archived()).To(BeFalse())
				})
			})
		})

		Context("pipeline does not have a parent job and build ID", func() {
			It("Should not archive the pipeline", func() {
				defaultPipeline.Reload()
				Expect(defaultPipeline.Archived()).To(BeFalse())
			})
		})
	})

	Describe("RemoveBuildEventsForDeletedPipelines", func() {
		var (
			pipeline1 db.Pipeline
			pipeline2 db.Pipeline
		)

		BeforeEach(func() {
			pipeline1, _, err = defaultTeam.SavePipeline(atc.PipelineRef{Name: "pipeline1"}, defaultPipelineConfig, 0, false)
			Expect(err).ToNot(HaveOccurred())
			pipeline2, _, err = defaultTeam.SavePipeline(atc.PipelineRef{Name: "pipeline2"}, defaultPipelineConfig, 0, false)
			Expect(err).ToNot(HaveOccurred())
		})

		pipelineBuildEventsExists := func(id int) bool {
			var exists bool
			err = dbConn.QueryRow(fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'pipeline_build_events_%d')", id)).Scan(&exists)
			Expect(err).ToNot(HaveOccurred())

			return exists
		}

		It("drops the pipeline_build_events_x table for each deleted pipeline", func() {
			destroy(pipeline1)
			destroy(pipeline2)

			err := pl.RemoveBuildEventsForDeletedPipelines()
			Expect(err).ToNot(HaveOccurred())

			Expect(pipelineBuildEventsExists(pipeline1.ID())).To(BeFalse())
			Expect(pipelineBuildEventsExists(pipeline2.ID())).To(BeFalse())
		})

		It("clears the deleted_pipelines table", func() {
			destroy(pipeline1)
			err := pl.RemoveBuildEventsForDeletedPipelines()
			Expect(err).ToNot(HaveOccurred())

			var count int
			err = dbConn.QueryRow("SELECT COUNT(*) FROM deleted_pipelines").Scan(&count)
			Expect(err).ToNot(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("is resilient to pipeline_build_events_x tables not existing", func() {
			destroy(pipeline1)
			_, err := dbConn.Exec(fmt.Sprintf("DROP TABLE pipeline_build_events_%d", pipeline1.ID()))
			Expect(err).ToNot(HaveOccurred())

			err = pl.RemoveBuildEventsForDeletedPipelines()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
