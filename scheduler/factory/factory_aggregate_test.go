package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Aggregate", func() {
	var (
		buildFactory *factory.BuildFactory

		resources atc.ResourceConfigs
	)

	BeforeEach(func() {
		buildFactory = &factory.BuildFactory{
			PipelineName: "some-pipeline",
		}

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
						Location: &atc.Location{
							ParentID:      0,
							ID:            3,
							ParallelGroup: 2,
						},
						Task: &atc.TaskPlan{
							Name: "some thing",
						},
					},
					{
						Location: &atc.Location{
							ParentID:      0,
							ID:            4,
							ParallelGroup: 2,
						},
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
						Location: &atc.Location{
							ParentID:      0,
							ID:            3,
							ParallelGroup: 2,
						},
						Task: &atc.TaskPlan{
							Name: "some thing",
						},
					},
					{
						Aggregate: &atc.AggregatePlan{
							{
								Location: &atc.Location{
									ParentID:      2,
									ID:            6,
									ParallelGroup: 5,
								},
								Task: &atc.TaskPlan{
									Name: "some nested thing",
								},
							},
							{
								Location: &atc.Location{
									ParentID:      2,
									ID:            7,
									ParallelGroup: 5,
								},
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
								Location: &atc.Location{
									ParentID:      0,
									ID:            3,
									ParallelGroup: 2,
								},
								Task: &atc.TaskPlan{
									Name: "some thing",
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{
									ParentID:      3,
									ID:            4,
									ParallelGroup: 0,
									Hook:          "success",
								},
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
								Location: &atc.Location{
									ParentID:      0,
									ID:            3,
									ParallelGroup: 2,
								},
								Task: &atc.TaskPlan{
									Name: "some thing",
								},
							},
						},
					},
					Next: atc.Plan{
						Location: &atc.Location{
							ParentID:      2,
							ID:            4,
							ParallelGroup: 0,
							Hook:          "success",
						},
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
