package factory_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/scheduler/factory"
	"github.com/concourse/concourse/atc/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Retry Step", func() {
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

	Context("when there is a task annotated with 'attempts'", func() {
		BeforeEach(func() {
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task:     "second task",
						Attempts: 3,
					},
				},
			})
		})
		It("builds correctly", func() {
			actual, err := buildFactory.Create(fakeJob, nil, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.RetryPlan{
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "second task",
					VersionedResourceTypes: resourceTypes,
				}),
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "second task",
					VersionedResourceTypes: resourceTypes,
				}),
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "second task",
					VersionedResourceTypes: resourceTypes,
				}),
			})

			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})

	Context("when there is a task annotated with 'attempts' and 'on_success'", func() {
		BeforeEach(func() {
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task:     "second task",
						Attempts: 3,
						Success: &atc.PlanConfig{
							Task: "second task",
						},
					},
				},
			})
		})
		It("builds correctly", func() {
			actual, err := buildFactory.Create(fakeJob, nil, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
				Step: expectedPlanFactory.NewPlan(atc.RetryPlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "second task",
						VersionedResourceTypes: resourceTypes,
					}),
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "second task",
						VersionedResourceTypes: resourceTypes,
					}),
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "second task",
						VersionedResourceTypes: resourceTypes,
					}),
				}),
				Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "second task",
					VersionedResourceTypes: resourceTypes,
				}),
			})

			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})
})
