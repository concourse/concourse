package factory_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/scheduler/factory"
	"github.com/concourse/concourse/atc/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Try Step", func() {
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

	Context("when there is a task wrapped in a try", func() {
		BeforeEach(func(){
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Try: &atc.PlanConfig{
							Task: "first task",
						},
					},
					{
						Task: "second task",
					},
				},
			})
		})
		It("builds correctly", func() {
			actual, err := buildFactory.Create(fakeJob, nil, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.DoPlan{
				expectedPlanFactory.NewPlan(atc.TryPlan{
					Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "first task",
						VersionedResourceTypes: resourceTypes,
					}),
				}),
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "second task",
					VersionedResourceTypes: resourceTypes,
				}),
			})

			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})

	Context("when the try also has a hook", func() {
		BeforeEach(func() {
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Try: &atc.PlanConfig{
							Task: "first task",
						},
						Success: &atc.PlanConfig{
							Task: "second task",
						},
					},
				},
			})
		})
		It("builds correctly", func() {
			actual, err := buildFactory.Create(fakeJob, nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
				Step: expectedPlanFactory.NewPlan(atc.TryPlan{
					Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name: "first task",
					}),
				}),
				Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name: "second task",
				}),
			})

			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})
})
