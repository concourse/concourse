package factory_test

import (
	"time"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/scheduler/factory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Timeout Step", func() {
	var (
		buildFactory *BuildFactory
	)

	BeforeEach(func() {
		buildFactory = &BuildFactory{
			PipelineName: "some-pipeline",
		}
	})

	Context("When there is a task with a timeout", func() {
		It("builds correctly", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task:    "first task",
						Timeout: atc.Duration(10 * time.Second),
					},
				},
			}, nil, nil)

			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				Timeout: &atc.TimeoutPlan{
					Duration: atc.Duration(10 * time.Second),
					Step: atc.Plan{
						Task: &atc.TaskPlan{
							Name: "first task",
						},
					},
				},
			}

			Ω(actual).Should(Equal(expected))
		})
	})
})
