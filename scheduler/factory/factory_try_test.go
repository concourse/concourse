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
				HookedCompose: &atc.HookedComposePlan{
					Step: atc.Plan{
						Try: &atc.TryPlan{
							Step: atc.Plan{
								Task: &atc.TaskPlan{
									Name: "first task",
								},
							},
						},
					},
					Next: atc.Plan{
						Task: &atc.TaskPlan{
							Name: "second task",
						},
					},
				},
			}

			Ω(actual).Should(Equal(expected))
		})
	})
})
