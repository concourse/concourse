package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"
	"github.com/concourse/atc/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Retry Step", func() {
	var (
		buildFactory        factory.BuildFactory
		actualPlanFactory   atc.PlanFactory
		expectedPlanFactory atc.PlanFactory
	)

	BeforeEach(func() {
		actualPlanFactory = atc.NewPlanFactory(123)
		expectedPlanFactory = atc.NewPlanFactory(123)
		buildFactory = factory.NewBuildFactory("some-pipeline", actualPlanFactory)
	})

	Context("when there is a task annotated with 'attempts'", func() {
		It("builds correctly", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task:     "second task",
						Attempts: 3,
					},
				},
			}, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.RetryPlan{
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "second task",
					Pipeline: "some-pipeline",
				}),
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "second task",
					Pipeline: "some-pipeline",
				}),
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "second task",
					Pipeline: "some-pipeline",
				}),
			})

			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})

	Context("when there is a task annotated with 'attempts' and 'on_success'", func() {
		It("builds correctly", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task:     "second task",
						Attempts: 3,
						Success: &atc.PlanConfig{
							Task: "second task",
						},
					},
				},
			}, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
				Step: expectedPlanFactory.NewPlan(atc.RetryPlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "second task",
						Pipeline: "some-pipeline",
					}),
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "second task",
						Pipeline: "some-pipeline",
					}),
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "second task",
						Pipeline: "some-pipeline",
					}),
				}),
				Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "second task",
					Pipeline: "some-pipeline",
				}),
			})

			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})
})
