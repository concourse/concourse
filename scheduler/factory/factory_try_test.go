package factory_test

import (
	"github.com/concourse/atc"
	. "github.com/concourse/atc/scheduler/factory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Try Step", func() {
	var (
		buildFactory *BuildFactory
	)

	BeforeEach(func() {
		buildFactory = &BuildFactory{
			PipelineName: "some-pipeline",
		}
	})

	Context("When there is a task wrapped in a try", func() {
		It("builds correctly", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
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

			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				OnSuccess: &atc.OnSuccessPlan{
					Step: atc.Plan{
						Try: &atc.TryPlan{
							Step: atc.Plan{
								Location: &atc.Location{
									ID:            2,
									ParentID:      0,
									ParallelGroup: 0,
								},
								Task: &atc.TaskPlan{
									Name: "first task",
								},
							},
						},
					},
					Next: atc.Plan{
						Location: &atc.Location{
							ID:            3,
							ParentID:      0,
							ParallelGroup: 0,
						},
						Task: &atc.TaskPlan{
							Name: "second task",
						},
					},
				},
			}

			Ω(actual).Should(Equal(expected))
		})
	})

	Context("When the try is in a hook", func() {
		It("builds correctly", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
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

			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				OnSuccess: &atc.OnSuccessPlan{
					Step: atc.Plan{
						Location: &atc.Location{
							ID:            1,
							ParentID:      0,
							ParallelGroup: 0,
						},
						Task: &atc.TaskPlan{
							Name: "first task",
						},
					},
					Next: atc.Plan{
						Try: &atc.TryPlan{
							Step: atc.Plan{
								Location: &atc.Location{
									ID:            3,
									ParentID:      1,
									ParallelGroup: 0,
									Hook:          "success",
								},
								Task: &atc.TaskPlan{
									Name: "second task",
								},
							},
						},
					},
				},
			}

			Ω(actual).Should(Equal(expected))
		})
	})
})
