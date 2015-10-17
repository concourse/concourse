package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"
	"github.com/concourse/atc/scheduler/factory/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Timeout Step", func() {
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

	Context("When there is a task with a timeout", func() {
		It("builds correctly", func() {
			actual := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task:    "first task",
						Timeout: "10s",
					},
				},
			}, nil, nil)

			expected := atc.Plan{
				Timeout: &atc.TimeoutPlan{
					Duration: "10s",
					Step: atc.Plan{
						Task: &atc.TaskPlan{
							Name:     "first task",
							Pipeline: "some-pipeline",
						},
					},
				},
			}

			Expect(actual).To(Equal(expected))
		})
	})
})
