package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Aggregate", func() {
	var (
		buildFactory factory.BuildFactory

		resources           atc.ResourceConfigs
		resourceTypes       atc.ResourceTypes
		actualPlanFactory   atc.PlanFactory
		expectedPlanFactory atc.PlanFactory
	)

	BeforeEach(func() {
		actualPlanFactory = atc.NewPlanFactory(123)
		expectedPlanFactory = atc.NewPlanFactory(123)

		buildFactory = factory.NewBuildFactory(42, actualPlanFactory)

		resources = atc.ResourceConfigs{
			{
				Name:   "some-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-resource"},
			},
		}

		resourceTypes = atc.ResourceTypes{
			{
				Name:   "some-custom-resource",
				Type:   "docker-image",
				Source: atc.Source{"some": "custom-source"},
			},
		}
	})

	Context("when I have one aggregate", func() {
		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
							},
							{
								Task: "some other thing",
							},
						},
					},
				},
			}, resources, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.AggregatePlan{
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:          "some thing",
					PipelineID:    42,
					ResourceTypes: resourceTypes,
				}),
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:          "some other thing",
					PipelineID:    42,
					ResourceTypes: resourceTypes,
				}),
			})
			Expect(actual).To(Equal(expected))
		})
	})

	Context("when I have nested aggregates", func() {
		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
							},
							{
								Aggregate: &atc.PlanSequence{
									{
										Task: "some nested thing",
									},
									{
										Task: "some nested other thing",
									},
								},
							},
						},
					},
				},
			}, resources, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.AggregatePlan{
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:          "some thing",
					PipelineID:    42,
					ResourceTypes: resourceTypes,
				}),
				expectedPlanFactory.NewPlan(atc.AggregatePlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:          "some nested thing",
						PipelineID:    42,
						ResourceTypes: resourceTypes,
					}),
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:          "some nested other thing",
						PipelineID:    42,
						ResourceTypes: resourceTypes,
					}),
				}),
			})
			Expect(actual).To(Equal(expected))
		})
	})

	Context("when I have an aggregate with hooks", func() {
		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
								CallbackConfig: atc.CallbackConfig{
									Success: &atc.PlanConfig{
										Task: "some success hook",
									},
								},
							},
						},
					},
				},
			}, resources, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.AggregatePlan{
				expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
					Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:          "some thing",
						PipelineID:    42,
						ResourceTypes: resourceTypes,
					}),
					Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:          "some success hook",
						PipelineID:    42,
						ResourceTypes: resourceTypes,
					}),
				}),
			})
			Expect(actual).To(Equal(expected))
		})
	})

	Context("when I have a hook on an aggregate", func() {
		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
							},
						},
						CallbackConfig: atc.CallbackConfig{
							Success: &atc.PlanConfig{
								Task: "some success hook",
							},
						},
					},
				},
			}, resources, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
				Step: expectedPlanFactory.NewPlan(atc.AggregatePlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:          "some thing",
						PipelineID:    42,
						ResourceTypes: resourceTypes,
					}),
				}),
				Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:          "some success hook",
					PipelineID:    42,
					ResourceTypes: resourceTypes,
				}),
			})
			Expect(actual).To(Equal(expected))
		})
	})
})
