package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"
	"github.com/concourse/atc/scheduler/factory/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Aggregate", func() {
	var (
		fakeLocationPopulator *fakes.FakeLocationPopulator
		buildFactory          factory.BuildFactory

		resources atc.ResourceConfigs
	)

	BeforeEach(func() {
		fakeLocationPopulator = &fakes.FakeLocationPopulator{}
		buildFactory = factory.NewBuildFactory(
			"some-pipeline",
			fakeLocationPopulator,
		)

		resources = atc.ResourceConfigs{
			{
				Name:   "some-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-resource"},
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
			}, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				Aggregate: &atc.AggregatePlan{
					{
						Task: &atc.TaskPlan{
							Name: "some thing",
						},
					},
					{
						Task: &atc.TaskPlan{
							Name: "some other thing",
						},
					},
				},
			}
			Ω(actual).Should(Equal(expected))
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
			}, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				Aggregate: &atc.AggregatePlan{
					{
						Task: &atc.TaskPlan{
							Name: "some thing",
						},
					},
					{
						Aggregate: &atc.AggregatePlan{
							{
								Task: &atc.TaskPlan{
									Name: "some nested thing",
								},
							},
							{
								Task: &atc.TaskPlan{
									Name: "some nested other thing",
								},
							},
						},
					},
				},
			}
			Ω(actual).Should(Equal(expected))
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
								Success: &atc.PlanConfig{
									Task: "some success hook",
								},
							},
						},
					},
				},
			}, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				Aggregate: &atc.AggregatePlan{
					{
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								Task: &atc.TaskPlan{
									Name: "some thing",
								},
							},
							Next: atc.Plan{
								Task: &atc.TaskPlan{
									Name: "some success hook",
								},
							},
						},
					},
				},
			}
			Ω(actual).Should(Equal(expected))
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
						Success: &atc.PlanConfig{
							Task: "some success hook",
						},
					},
				},
			}, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				OnSuccess: &atc.OnSuccessPlan{
					Step: atc.Plan{
						Aggregate: &atc.AggregatePlan{
							{
								Task: &atc.TaskPlan{
									Name: "some thing",
								},
							},
						},
					},
					Next: atc.Plan{
						Task: &atc.TaskPlan{
							Name: "some success hook",
						},
					},
				},
			}
			Ω(actual).Should(Equal(expected))
		})
	})
})
