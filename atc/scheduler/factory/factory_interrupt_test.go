package factory_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/scheduler/factory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Interrupt Step", func() {
	var (
		resourceTypes atc.VersionedResourceTypes

		buildFactory        factory.BuildFactory
		actualPlanFactory   atc.PlanFactory
		expectedPlanFactory atc.PlanFactory
	)

	BeforeEach(func() {
		actualPlanFactory = atc.NewPlanFactory(321)
		expectedPlanFactory = atc.NewPlanFactory(321)
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

	Context("When there is a task with an interrupt", func() {
		It("builds correctly", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task:      "first task",
						Interrupt: "5m",
					},
				},
			}, nil, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.InterruptPlan{
				Duration: "5m",
				Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "first task",
					VersionedResourceTypes: resourceTypes,
				}),
			})

			Expect(actual).To(Equal(expected))
		})
	})
})
