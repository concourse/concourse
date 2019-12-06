package factory_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/scheduler/factory"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Parallel", func() {
	var (
		buildFactory factory.BuildFactory

		resources           atc.ResourceConfigs
		resourceTypes       atc.VersionedResourceTypes
		actualPlanFactory   atc.PlanFactory
		expectedPlanFactory atc.PlanFactory
		fakeJob             *dbfakes.FakeJob
	)

	BeforeEach(func() {
		actualPlanFactory = atc.NewPlanFactory(123)
		expectedPlanFactory = atc.NewPlanFactory(123)

		fakeJob = new(dbfakes.FakeJob)

		buildFactory = factory.NewBuildFactory(actualPlanFactory)

		resources = atc.ResourceConfigs{
			{
				Name:   "some-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-resource"},
			},
		}

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

	Context("when I have a parallel step", func() {
		BeforeEach(func() {
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						InParallel: &atc.InParallelConfig{
							Steps: atc.PlanSequence{
								{
									Task: "some thing",
								},
								{
									Task: "some other thing",
								},
							},
							Limit:    1,
							FailFast: true,
						},
					},
				},
			})
		})
		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(fakeJob, resources, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.InParallelPlan{
				Steps: []atc.Plan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "some thing",
						VersionedResourceTypes: resourceTypes,
					}),
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "some other thing",
						VersionedResourceTypes: resourceTypes,
					}),
				},
				Limit:    1,
				FailFast: true,
			})
			Expect(actual).To(Equal(expected))
		})
	})
})
