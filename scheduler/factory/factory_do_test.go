package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"
	"github.com/concourse/atc/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Do", func() {
	var (
		buildFactory factory.BuildFactory

		resources           atc.ResourceConfigs
		actualPlanFactory   atc.PlanFactory
		expectedPlanFactory atc.PlanFactory
	)

	BeforeEach(func() {
		actualPlanFactory = atc.NewPlanFactory(123)
		expectedPlanFactory = atc.NewPlanFactory(123)

		buildFactory = factory.NewBuildFactory("some-pipeline", actualPlanFactory)

		resources = atc.ResourceConfigs{
			{
				Name:   "some-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-resource"},
			},
		}
	})

	Context("when I have a nested do ", func() {
		It("returns the correct plan", func() {
			actual := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Do: &atc.PlanSequence{
							{
								Task: "some thing",
							},
							{
								Task: "some thing-2",
							},
							{
								Do: &atc.PlanSequence{
									{
										Task: "some other thing",
									},
								},
							},
						},
					},
				},
			}, resources, nil)

			expected := expectedPlanFactory.NewPlan(atc.DoPlan{
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "some thing",
					Pipeline: "some-pipeline",
				}),
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "some thing-2",
					Pipeline: "some-pipeline",
				}),
				expectedPlanFactory.NewPlan(atc.DoPlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "some other thing",
						Pipeline: "some-pipeline",
					}),
				}),
			})
			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})

	Context("when I have an aggregate inside a do", func() {
		It("returns the correct plan", func() {
			actual := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Do: &atc.PlanSequence{
							{
								Task: "some thing",
							},
							{
								Aggregate: &atc.PlanSequence{
									{
										Task: "some other thing",
									},
								},
							},
							{
								Task: "some thing-2",
							},
						},
					},
				},
			}, resources, nil)

			expected := expectedPlanFactory.NewPlan(atc.DoPlan{
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "some thing",
					Pipeline: "some-pipeline",
				}),
				expectedPlanFactory.NewPlan(atc.AggregatePlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "some other thing",
						Pipeline: "some-pipeline",
					}),
				}),
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "some thing-2",
					Pipeline: "some-pipeline",
				}),
			})
			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})

	Context("when i have a do inside an aggregate inside a hook", func() {
		It("returns the correct plan", func() {
			actual := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "starting-task",
						Success: &atc.PlanConfig{
							Aggregate: &atc.PlanSequence{
								{
									Task: "some thing",
								},
								{
									Do: &atc.PlanSequence{
										{
											Task: "some other thing",
										},
									},
								},
							},
						},
					},
				},
			}, resources, nil)

			expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
				Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "starting-task",
					Pipeline: "some-pipeline",
				}),
				Next: expectedPlanFactory.NewPlan(atc.AggregatePlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "some thing",
						Pipeline: "some-pipeline",
					}),
					expectedPlanFactory.NewPlan(atc.DoPlan{
						expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:     "some other thing",
							Pipeline: "some-pipeline",
						}),
					}),
				}),
			})

			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})

	Context("when I have a do inside an aggregate", func() {
		It("returns the correct plan", func() {
			actual := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
							},
							{
								Do: &atc.PlanSequence{
									{
										Task: "some other thing",
									},
									{
										Task: "some other thing-2",
									},
								},
							},
							{
								Task: "some thing-2",
							},
						},
					},
				},
			}, resources, nil)

			expected := expectedPlanFactory.NewPlan(atc.AggregatePlan{
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "some thing",
					Pipeline: "some-pipeline",
				}),
				expectedPlanFactory.NewPlan(atc.DoPlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "some other thing",
						Pipeline: "some-pipeline",
					}),
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "some other thing-2",
						Pipeline: "some-pipeline",
					}),
				}),
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "some thing-2",
					Pipeline: "some-pipeline",
				}),
			})

			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})
})
