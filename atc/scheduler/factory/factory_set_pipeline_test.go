package factory_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/scheduler/factory"
	"github.com/concourse/concourse/atc/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory SetPipeline Step", func() {
	var (
		resourceTypes atc.VersionedResourceTypes

		buildFactory        factory.BuildFactory
		actualPlanFactory   atc.PlanFactory
		expectedPlanFactory atc.PlanFactory
		fakeJob             *dbfakes.FakeJob
	)

	BeforeEach(func() {
		actualPlanFactory = atc.NewPlanFactory(123)
		expectedPlanFactory = atc.NewPlanFactory(123)
		buildFactory = factory.NewBuildFactory(actualPlanFactory)
		fakeJob = new(dbfakes.FakeJob)

		resourceTypes = atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "some-custom-resource",
					Type:   "registry-image",
					Source: atc.Source{"some": "custom-source"},
				},
				Version: atc.Version{"some": "version"},
			},
		}
	})

	Context("when set other pipeline", func() {
		BeforeEach(func() {
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						SetPipeline: "some-pipeline",
						ConfigPath:  "some-file",
						VarFiles:    []string{"vf1", "vf2"},
						Vars:        map[string]interface{}{"k1": "v1"},
					},
				},
			})
		})
		It("builds correctly", func() {
			actual, err := buildFactory.Create(fakeJob, nil, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.SetPipelinePlan{
				Name:     "some-pipeline",
				File:     "some-file",
				VarFiles: []string{"vf1", "vf2"},
				Vars:     map[string]interface{}{"k1": "v1"},
			})

			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})

	Context("when set self", func() {
		BeforeEach(func() {
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						SetPipeline: "self",
						ConfigPath:  "some-file",
						VarFiles:    []string{"vf1", "vf2"},
						Vars:        map[string]interface{}{"k1": "v1"},
					},
				},
			})

			fakeJob.PipelineNameReturns("self-pipeline")
		})
		It("builds correctly", func() {
			actual, err := buildFactory.Create(fakeJob, nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.SetPipelinePlan{
				Name:     "self-pipeline",
				File:     "some-file",
				VarFiles: []string{"vf1", "vf2"},
				Vars:     map[string]interface{}{"k1": "v1"},
			})

			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})
})
