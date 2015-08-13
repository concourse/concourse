package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Do", func() {
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

	Context("when I have a nested do ", func() {
		It("returns the correct plan", func() {

			actual, err := buildFactory.Create(atc.JobConfig{
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
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				OnSuccess: &atc.OnSuccessPlan{
					Step: atc.Plan{
						Location: &atc.Location{
							ParentID:      0,
							ID:            3,
							ParallelGroup: 0,
							SerialGroup:   2,
						},
						Task: &atc.TaskPlan{
							Name: "some thing",
						},
					},
					Next: atc.Plan{
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								Location: &atc.Location{
									ParentID:      0,
									ID:            4,
									ParallelGroup: 0,
									SerialGroup:   2,
								},
								Task: &atc.TaskPlan{
									Name: "some thing-2",
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{
									ParentID:      2,
									ID:            7,
									ParallelGroup: 0,
									SerialGroup:   6,
								},
								Task: &atc.TaskPlan{
									Name: "some other thing",
								},
							},
						},
					},
				},
			}
			Ω(actual).Should(Equal(expected))
		})
	})

	Context("when I have an aggregate inside a do", func() {
		It("returns the correct plan", func() {

			actual, err := buildFactory.Create(atc.JobConfig{
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
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				OnSuccess: &atc.OnSuccessPlan{
					Step: atc.Plan{
						Location: &atc.Location{
							ParentID:      0,
							ID:            3,
							ParallelGroup: 0,
							SerialGroup:   2,
						},
						Task: &atc.TaskPlan{
							Name: "some thing",
						},
					},
					Next: atc.Plan{
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{

								Aggregate: &atc.AggregatePlan{
									{
										Location: &atc.Location{
											ParentID:      0,
											ID:            6,
											ParallelGroup: 5,
											SerialGroup:   2,
										},
										Task: &atc.TaskPlan{
											Name: "some other thing",
										},
									},
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{
									ParentID:      0,
									ID:            7,
									ParallelGroup: 0,
									SerialGroup:   2,
								},
								Task: &atc.TaskPlan{
									Name: "some thing-2",
								},
							},
						},
					},
				},
			}
			Ω(actual).Should(Equal(expected))
		})
	})

	Context("when i have a do inside an aggregate inside a hook", func() {
		It("returns the correct plan", func() {

			actual, err := buildFactory.Create(atc.JobConfig{
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
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				OnSuccess: &atc.OnSuccessPlan{
					Step: atc.Plan{
						Location: &atc.Location{
							ParentID:      0,
							ID:            1,
							ParallelGroup: 0,
							SerialGroup:   0,
						},
						Task: &atc.TaskPlan{
							Name: "starting-task",
						},
					},
					Next: atc.Plan{
						Aggregate: &atc.AggregatePlan{
							{
								Location: &atc.Location{
									ParentID:      1,
									ID:            4,
									ParallelGroup: 3,
									SerialGroup:   0,
									Hook:          "success",
								},
								Task: &atc.TaskPlan{
									Name: "some thing",
								},
							},
							{
								Location: &atc.Location{
									ParentID:      1,
									ID:            7,
									ParallelGroup: 3,
									SerialGroup:   6,
								},
								Task: &atc.TaskPlan{
									Name: "some other thing",
								},
							},
						},
					},
				},
			}

			Ω(actual).Should(Equal(expected))
		})
	})

	Context("when I have a do inside an aggregate", func() {
		It("returns the correct plan", func() {

			actual, err := buildFactory.Create(atc.JobConfig{
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
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				Aggregate: &atc.AggregatePlan{
					{
						Location: &atc.Location{
							ParentID:      0,
							ID:            3,
							ParallelGroup: 2,
							SerialGroup:   0,
						},
						Task: &atc.TaskPlan{
							Name: "some thing",
						},
					},
					{
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								Location: &atc.Location{
									ParentID:      0,
									ID:            6,
									ParallelGroup: 2,
									SerialGroup:   5,
								},
								Task: &atc.TaskPlan{
									Name: "some other thing",
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{
									ParentID:      0,
									ID:            7,
									ParallelGroup: 2,
									SerialGroup:   5,
								},
								Task: &atc.TaskPlan{
									Name: "some other thing-2",
								},
							},
						},
					},
					{
						Location: &atc.Location{
							ParentID:      0,
							ID:            8,
							ParallelGroup: 2,
							SerialGroup:   0,
						},
						Task: &atc.TaskPlan{
							Name: "some thing-2",
						},
					},
				},
			}

			Ω(actual).Should(Equal(expected))
		})
	})
})
