package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Put", func() {
	var (
		buildFactory *factory.BuildFactory

		resources atc.ResourceConfigs

		input atc.JobConfig
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

	Context("when I have a put at the top-level", func() {
		BeforeEach(func() {
			input = atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Put:      "some-put",
						Resource: "some-resource",
					},
				},
			}
		})

		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(input, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				OnSuccess: &atc.OnSuccessPlan{
					Step: atc.Plan{
						Location: &atc.Location{
							ParentID:      0,
							ID:            1,
							ParallelGroup: 0,
						},
						Put: &atc.PutPlan{
							Type:     "git",
							Name:     "some-put",
							Resource: "some-resource",
							Pipeline: "some-pipeline",
							Source: atc.Source{
								"uri": "git://some-resource",
							},
						},
					},
					Next: atc.Plan{
						Location: &atc.Location{
							ParentID:      1,
							ID:            2,
							ParallelGroup: 0,
						},
						DependentGet: &atc.DependentGetPlan{
							Type:     "git",
							Name:     "some-put",
							Resource: "some-resource",
							Pipeline: "some-pipeline",
							Source: atc.Source{
								"uri": "git://some-resource",
							},
						},
					},
				},
			}
			Ω(actual).Should(Equal(expected))
		})
	})

	Context("when I have a put in a hook", func() {
		BeforeEach(func() {
			input = atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "some-task",
						Success: &atc.PlanConfig{
							Put: "some-put",
						},
					},
				},
			}
		})

		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(input, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				OnSuccess: &atc.OnSuccessPlan{
					Step: atc.Plan{
						Location: &atc.Location{
							ParentID:      0,
							ID:            1,
							ParallelGroup: 0,
						},
						Task: &atc.TaskPlan{
							Name: "some-task",
						},
					},

					Next: atc.Plan{
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								Location: &atc.Location{
									ParentID:      1,
									ID:            2,
									ParallelGroup: 0,
									Hook:          "success",
								},
								Put: &atc.PutPlan{
									Name:     "some-put",
									Resource: "some-put",
									Pipeline: "some-pipeline",
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{
									ParentID:      2,
									ID:            3,
									ParallelGroup: 0,
								},
								DependentGet: &atc.DependentGetPlan{
									Name:     "some-put",
									Resource: "some-put",
									Pipeline: "some-pipeline",
								},
							},
						},
					},
				},
			}
			Ω(actual).Should(Equal(expected))
		})
	})

	Context("when I have a put inside an aggregate", func() {
		BeforeEach(func() {
			input = atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
							},
							{
								Put: "some-put",
							},
						},
					},
				},
			}
		})

		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(input, resources, nil)
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
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								Location: &atc.Location{
									ParentID:      0,
									ID:            4,
									ParallelGroup: 2,
								},
								Put: &atc.PutPlan{
									Name:     "some-put",
									Resource: "some-put",
									Pipeline: "some-pipeline",
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{
									ParentID:      4,
									ID:            5,
									ParallelGroup: 0,
								},
								DependentGet: &atc.DependentGetPlan{
									Name:     "some-put",
									Resource: "some-put",
									Pipeline: "some-pipeline",
								},
							},
						},
					},
				},
			}
			Ω(actual).Should(Equal(expected))
		})
	})

	Context("when a put plan follows a task plan", func() {
		Context("with no explicit condition", func() {
			BeforeEach(func() {
				input = atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Task: "some-task",
						},
						{
							Put:      "money",
							Resource: "power",
						},
					},
				}
			})

			It("runs only on success", func() {
				actual, err := buildFactory.Create(input, resources, nil)
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
								Name: "some-task",
							},
						},
						Next: atc.Plan{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									Location: &atc.Location{
										ID:            2,
										ParentID:      0,
										ParallelGroup: 0,
									},
									Put: &atc.PutPlan{
										Name:     "money",
										Resource: "power",
										Pipeline: "some-pipeline",
									},
								},
								Next: atc.Plan{
									Location: &atc.Location{
										ID:            3,
										ParentID:      2,
										ParallelGroup: 0,
									},

									DependentGet: &atc.DependentGetPlan{
										Name:     "money",
										Resource: "power",
										Pipeline: "some-pipeline",
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

	Context("when a put plan follows a task plan", func() {
		Context("with no explicit condition", func() {
			var input atc.JobConfig

			BeforeEach(func() {
				input = atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Task: "those who resist our will",
						},
						{
							Put: "some-other-other-resource",
						},
						{
							Task: "some-other-task",
						},
					},
				}
			})
			It("runs only on success", func() {
				expectedPlan := atc.Plan{
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							Task: &atc.TaskPlan{
								Name: "those who resist our will",
							},
							Location: &atc.Location{
								ID:            1,
								ParentID:      0,
								ParallelGroup: 0,
							},
						},
						Next: atc.Plan{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									OnSuccess: &atc.OnSuccessPlan{
										Step: atc.Plan{
											Location: &atc.Location{
												ID:            2,
												ParentID:      0,
												ParallelGroup: 0,
											},
											Put: &atc.PutPlan{
												Name:     "some-other-other-resource",
												Resource: "some-other-other-resource",
												Pipeline: "some-pipeline",
												Params:   nil,
											},
										},
										Next: atc.Plan{
											Location: &atc.Location{
												ID:            3,
												ParentID:      2,
												ParallelGroup: 0,
											},
											DependentGet: &atc.DependentGetPlan{
												Name:     "some-other-other-resource",
												Resource: "some-other-other-resource",
												Pipeline: "some-pipeline",
											},
										},
									},
								},
								Next: atc.Plan{
									Location: &atc.Location{
										ID:            4,
										ParentID:      0,
										ParallelGroup: 0,
									},
									Task: &atc.TaskPlan{
										Name: "some-other-task",
									},
								},
							},
						},
					},
				}

				builtPlan, err := buildFactory.Create(input, resources, nil)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(builtPlan).Should(Equal(expectedPlan))
			})
		})
	})

})
