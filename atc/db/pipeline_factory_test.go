package db_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipeline Factory", func() {
	var pipelineFactory db.PipelineFactory

	BeforeEach(func() {
		pipelineFactory = db.NewPipelineFactory(dbConn, lockFactory)
	})

	Describe("VisiblePipelines", func() {
		var (
			pipeline1 db.Pipeline
			pipeline2 db.Pipeline
			pipeline3 db.Pipeline
			pipeline4 db.Pipeline
			team      db.Team
		)

		BeforeEach(func() {
			err := defaultPipeline.Destroy()
			Expect(err).ToNot(HaveOccurred())

			team, err = teamFactory.CreateTeam(atc.Team{Name: "some-team"})
			Expect(err).ToNot(HaveOccurred())

			pipeline1, _, err = team.SavePipeline(atc.PipelineRef{Name: "fake-pipeline"}, atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-name"},
				},
			}, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline1.Reload()).To(BeTrue())

			pipeline2, _, err = defaultTeam.SavePipeline(atc.PipelineRef{Name: "fake-pipeline-two"}, atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-fake"},
				},
			}, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline2.Reload()).To(BeTrue())

			pipeline3, _, err = defaultTeam.SavePipeline(atc.PipelineRef{Name: "fake-pipeline-three"}, atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-fake-two"},
				},
			}, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline3.Expose()).To(Succeed())
			Expect(pipeline3.Reload()).To(BeTrue())

			pipeline4, _, err = team.SavePipeline(atc.PipelineRef{Name: "fake-pipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}, atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-name"},
				},
			}, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline4.Reload()).To(BeTrue())
		})

		It("returns all pipelines visible for the given teams", func() {
			pipelines, err := pipelineFactory.VisiblePipelines([]string{"some-team"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pipelineRefs(pipelines)).To(Equal([]atc.PipelineRef{
				pipelineRef(pipeline1),
				pipelineRef(pipeline4),
				pipelineRef(pipeline3),
			}))
		})

		It("returns all pipelines visible when empty team name provided", func() {
			pipelines, err := pipelineFactory.VisiblePipelines([]string{""})
			Expect(err).ToNot(HaveOccurred())
			Expect(pipelineRefs(pipelines)).To(Equal([]atc.PipelineRef{
				pipelineRef(pipeline3),
			}))
		})

		It("returns all pipelines visible when empty teams provided", func() {
			pipelines, err := pipelineFactory.VisiblePipelines([]string{})
			Expect(err).ToNot(HaveOccurred())
			Expect(pipelineRefs(pipelines)).To(Equal([]atc.PipelineRef{
				pipelineRef(pipeline3),
			}))
		})

		It("returns all pipelines visible when nil teams provided", func() {
			pipelines, err := pipelineFactory.VisiblePipelines(nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipelineRefs(pipelines)).To(Equal([]atc.PipelineRef{
				pipelineRef(pipeline3),
			}))
		})

		Describe("When instance pipeline ordered is change", func() {
			BeforeEach(func() {
				err := team.OrderPipelinesWithinGroup("fake-pipeline", []atc.InstanceVars{
					{"branch": "master"},
					{},
				})
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should keep the right order", func() {
				pipelines, err := pipelineFactory.VisiblePipelines([]string{"some-team"})
				Expect(err).ToNot(HaveOccurred())
				Expect(pipelineRefs(pipelines)).To(Equal([]atc.PipelineRef{
					pipelineRef(pipeline4),
					pipelineRef(pipeline1),
					pipelineRef(pipeline3),
				}))
			})
		})
	})

	Describe("AllPipelines", func() {
		var (
			team      db.Team
			pipeline1 db.Pipeline
			pipeline2 db.Pipeline
			pipeline3 db.Pipeline
			pipeline4 db.Pipeline
		)

		BeforeEach(func() {
			err := defaultPipeline.Destroy()
			Expect(err).ToNot(HaveOccurred())

			team, err = teamFactory.CreateTeam(atc.Team{Name: "some-team"})
			Expect(err).ToNot(HaveOccurred())

			pipeline2, _, err = team.SavePipeline(atc.PipelineRef{Name: "fake-pipeline-two"}, atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-fake"},
				},
			}, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline2.Reload()).To(BeTrue())

			pipeline3, _, err = team.SavePipeline(atc.PipelineRef{Name: "fake-pipeline-three"}, atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-fake-two"},
				},
			}, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline3.Expose()).To(Succeed())
			Expect(pipeline3.Reload()).To(BeTrue())

			pipeline1, _, err = defaultTeam.SavePipeline(atc.PipelineRef{Name: "fake-pipeline"}, atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-name"},
				},
			}, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline1.Expose()).To(Succeed())
			Expect(pipeline1.Reload()).To(BeTrue())

			pipeline4, _, err = team.SavePipeline(atc.PipelineRef{Name: "fake-pipeline-two", InstanceVars: atc.InstanceVars{"branch": "master"}}, atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "job-name"},
				},
			}, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline4.Reload()).To(BeTrue())

		})

		It("returns all pipelines ordered by team id -> ordering -> secondary_ordering", func() {
			pipelines, err := pipelineFactory.AllPipelines()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipelineRefs(pipelines)).To(Equal([]atc.PipelineRef{
				pipelineRef(pipeline1),
				pipelineRef(pipeline2),
				pipelineRef(pipeline4),
				pipelineRef(pipeline3),
			}))
		})

		Describe("When instance pipeline ordered is change", func() {
			BeforeEach(func() {
				err := team.OrderPipelinesWithinGroup("fake-pipeline-two", []atc.InstanceVars{
					{"branch": "master"},
					{},
				})
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should keep the right order", func() {
				pipelines, err := pipelineFactory.AllPipelines()
				Expect(err).ToNot(HaveOccurred())
				Expect(pipelineRefs(pipelines)).To(Equal([]atc.PipelineRef{
					pipelineRef(pipeline1),
					pipelineRef(pipeline4),
					pipelineRef(pipeline2),
					pipelineRef(pipeline3),
				}))
			})
		})
	})
})

func pipelineRef(pipeline db.Pipeline) atc.PipelineRef {
	return atc.PipelineRef{Name: pipeline.Name(), InstanceVars: pipeline.InstanceVars()}
}

func pipelineRefs(pipelines []db.Pipeline) []atc.PipelineRef {
	refs := make([]atc.PipelineRef, len(pipelines))
	for i, p := range pipelines {
		refs[i] = pipelineRef(p)
	}
	return refs
}
