package db_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipeline Factory", func() {
	var pipelineFactory db.PipelineFactory

	BeforeEach(func() {
		pipelineFactory = db.NewPipelineFactory(dbConn, lockFactory)
	})

	Describe("PublicPipelines", func() {
		var (
			publicPipelines []db.Pipeline
			pipeline1       db.Pipeline
			pipeline2       db.Pipeline
			pipeline3       db.Pipeline
		)

		BeforeEach(func() {
			publicPipelines = nil

			team, err := teamFactory.CreateTeam(atc.Team{Name: "some-team"})
			Expect(err).ToNot(HaveOccurred())

			pipeline1, _, err = team.SavePipeline("fake-pipeline", atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-name"},
				},
			}, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline1.Expose()).To(Succeed())
			Expect(pipeline1.Reload()).To(BeTrue())

			pipeline2, _, err = defaultTeam.SavePipeline("fake-pipeline-two", atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-fake"},
				},
			}, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline2.Reload()).To(BeTrue())

			pipeline3, _, err = defaultTeam.SavePipeline("fake-pipeline-three", atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-fake-two"},
				},
			}, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline3.Expose()).To(Succeed())
			Expect(pipeline3.Reload()).To(BeTrue())
		})

		JustBeforeEach(func() {
			var err error
			publicPipelines, err = pipelineFactory.PublicPipelines()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns all public pipelines", func() {
			Expect(len(publicPipelines)).To(Equal(2))
			Expect(publicPipelines[0].Name()).To(Equal(pipeline3.Name()))
			Expect(publicPipelines[1].Name()).To(Equal(pipeline1.Name()))
		})

		Context("when a pipeline is hidden", func() {
			BeforeEach(func() {
				Expect(pipeline1.Hide()).To(Succeed())
			})

			It("returns only the remaining exposed pipeline", func() {
				Expect(len(publicPipelines)).To(Equal(1))
				Expect(publicPipelines[0].Name()).To(Equal(pipeline3.Name()))
			})
		})
	})

	Describe("AllPipelines", func() {
		var (
			pipeline1 db.Pipeline
			pipeline2 db.Pipeline
			pipeline3 db.Pipeline
		)

		BeforeEach(func() {
			err := defaultPipeline.Destroy()
			Expect(err).ToNot(HaveOccurred())

			team, err := teamFactory.CreateTeam(atc.Team{Name: "some-team"})
			Expect(err).ToNot(HaveOccurred())

			pipeline1, _, err = team.SavePipeline("fake-pipeline", atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-name"},
				},
			}, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline1.Expose()).To(Succeed())
			Expect(pipeline1.Reload()).To(BeTrue())

			pipeline2, _, err = defaultTeam.SavePipeline("fake-pipeline-two", atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-fake"},
				},
			}, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline2.Reload()).To(BeTrue())

			pipeline3, _, err = defaultTeam.SavePipeline("fake-pipeline-three", atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-fake-two"},
				},
			}, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline3.Expose()).To(Succeed())
			Expect(pipeline3.Reload()).To(BeTrue())
		})

		It("returns all pipelines", func() {
			pipelines, err := pipelineFactory.AllPipelines()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(pipelines)).To(Equal(3))
			Expect(pipelines[0].Name()).To(Equal(pipeline1.Name()))
			Expect(pipelines[1].Name()).To(Equal(pipeline2.Name()))
			Expect(pipelines[2].Name()).To(Equal(pipeline3.Name()))
		})
	})
})
