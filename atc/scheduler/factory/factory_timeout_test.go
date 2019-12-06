package factory_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/scheduler/factory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Timeout Step", func() {
	var (
		resourceTypes atc.VersionedResourceTypes

		buildFactory        factory.BuildFactory
		actualPlanFactory   atc.PlanFactory
		expectedPlanFactory atc.PlanFactory
		fakeJob             *dbfakes.FakeJob
	)

	BeforeEach(func() {
		actualPlanFactory = atc.NewPlanFactory(321)
		expectedPlanFactory = atc.NewPlanFactory(321)
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

	Context("When there is a task with a timeout", func() {
		BeforeEach(func() {
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task:    "first task",
						Timeout: "10s",
					},
				},
			})
		})
		It("builds correctly", func() {
			actual, err := buildFactory.Create(fakeJob, nil, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.TimeoutPlan{
				Duration: "10s",
				Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "first task",
					VersionedResourceTypes: resourceTypes,
				}),
			})

			Expect(actual).To(Equal(expected))
		})
	})
})
