package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Timeout Step", func() {
	var (
		buildFactory        factory.BuildFactory
		actualPlanFactory   atc.PlanFactory
		expectedPlanFactory atc.PlanFactory
	)

	BeforeEach(func() {
		actualPlanFactory = atc.NewPlanFactory(321)
		expectedPlanFactory = atc.NewPlanFactory(321)
		buildFactory = factory.NewBuildFactory("some-pipeline", actualPlanFactory)
	})

	Context("When there is a task with a timeout", func() {
		It("builds correctly", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task:    "first task",
						Timeout: "10s",
					},
				},
			}, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.TimeoutPlan{
				Duration: "10s",
				Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "first task",
					Pipeline: "some-pipeline",
				}),
			})

			Expect(actual).To(Equal(expected))
		})
	})
})
