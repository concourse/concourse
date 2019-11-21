package factory_test

import (
	"github.com/concourse/concourse/atc"
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
		input               atc.JobConfig
	)

	BeforeEach(func() {
		actualPlanFactory = atc.NewPlanFactory(123)
		expectedPlanFactory = atc.NewPlanFactory(123)
		buildFactory = factory.NewBuildFactory(actualPlanFactory)

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
			input = atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						SetPipeline: "some-pipeline",
						ConfigPath:  "some-file",
						VarFiles:    []string{"vf1", "vf2"},
						Vars:        map[string]interface{}{"k1": "v1"},
					},
				},
			}
		})
		It("builds correctly", func() {
			actual, err := buildFactory.Create(input, nil, resourceTypes, nil)
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
})
