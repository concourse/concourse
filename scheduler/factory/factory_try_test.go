package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"
	"github.com/concourse/atc/scheduler/factory/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Try Step", func() {
	var (
		fakeLocationPopulator *fakes.FakeLocationPopulator
		buildFactory          factory.BuildFactory
	)

	BeforeEach(func() {
		fakeLocationPopulator = &fakes.FakeLocationPopulator{}

		buildFactory = factory.NewBuildFactory(
			"some-pipeline",
			fakeLocationPopulator,
		)
	})

	Context("When there is a task wrapped in a try", func() {
		It("builds correctly", func() {
			actual := buildFactory.Create(atc.JobConfig{
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
			}, nil, nil)

			expected := atc.Plan{
				OnSuccess: &atc.OnSuccessPlan{
					Step: atc.Plan{
						Try: &atc.TryPlan{
							Step: atc.Plan{
								Task: &atc.TaskPlan{
									Name:     "first task",
									Pipeline: "some-pipeline",
								},
							},
						},
					},
					Next: atc.Plan{
						Task: &atc.TaskPlan{
							Name:     "second task",
							Pipeline: "some-pipeline",
						},
					},
				},
			}

			Expect(actual).To(Equal(expected))
		})
	})

	Context("When the try is in a hook", func() {
		It("builds correctly", func() {
			actual := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "first task",
						Success: &atc.PlanConfig{
							Try: &atc.PlanConfig{
								Task: "second task",
							},
						},
					},
				},
			}, nil, nil)

			expected := atc.Plan{
				OnSuccess: &atc.OnSuccessPlan{
					Step: atc.Plan{
						Task: &atc.TaskPlan{
							Name:     "first task",
							Pipeline: "some-pipeline",
						},
					},
					Next: atc.Plan{
						Try: &atc.TryPlan{
							Step: atc.Plan{
								Task: &atc.TaskPlan{
									Name:     "second task",
									Pipeline: "some-pipeline",
								},
							},
						},
					},
				},
			}

			Expect(actual).To(Equal(expected))
		})
	})
})
