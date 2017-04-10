package dbng_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipeline Factory", func() {
	var pipelineFactory dbng.PipelineFactory

	BeforeEach(func() {
		pipelineFactory = dbng.NewPipelineFactory(dbConn, lockFactory)
	})

	Describe("PublicPipelines", func() {
		var (
			publicPipelines []dbng.Pipeline
			pipeline1       dbng.Pipeline
			pipeline2       dbng.Pipeline
			pipeline3       dbng.Pipeline
		)

		BeforeEach(func() {
			publicPipelines = nil

			team, err := teamFactory.CreateTeam(atc.Team{Name: "some-team"})
			Expect(err).ToNot(HaveOccurred())

			pipeline1, _, err = team.SavePipeline("fake-pipeline", atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-name"},
				},
			}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline1.Expose()).To(Succeed())
			Expect(pipeline1.Reload()).To(BeTrue())

			pipeline2, _, err = defaultTeam.SavePipeline("fake-pipeline-two", atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-fake"},
				},
			}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			pipeline3, _, err = defaultTeam.SavePipeline("fake-pipeline-three", atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-fake-two"},
				},
			}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
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
			Expect(publicPipelines).To(Equal([]dbng.Pipeline{pipeline3, pipeline1}))
		})
	})
})
