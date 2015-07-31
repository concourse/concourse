package factory_test

import (
	"github.com/concourse/atc"
	. "github.com/concourse/atc/scheduler/factory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Hooks", func() {
	var (
		buildFactory *BuildFactory

		resources atc.ResourceConfigs
	)

	BeforeEach(func() {
		buildFactory = &BuildFactory{
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

	Context("when there is a do with a hook", func() {
		var input atc.JobConfig

		BeforeEach(func() {
			input = atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Do: &atc.PlanSequence{
							{
								Task: "those who resist our will",
							},
							{
								Task: "those who also resist our will",
							},
						},
						Failure: &atc.PlanConfig{
							Task: "some other failure",
						},
					},
				},
			}
		})

		It("builds the plan correctly", func() {
			actual, err := buildFactory.Create(input, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				OnFailure: &atc.OnFailurePlan{
					Step: atc.Plan{
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								Location: &atc.Location{
									ParentID:      0,
									ID:            2,
									ParallelGroup: 0,
								},
								Task: &atc.TaskPlan{
									Name: "those who resist our will",
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{
									ParentID:      0,
									ID:            3,
									ParallelGroup: 0,
								},
								Task: &atc.TaskPlan{
									Name: "those who also resist our will",
								},
							},
						},
					},
					Next: atc.Plan{
						Location: &atc.Location{
							ParentID:      2,
							ID:            4,
							ParallelGroup: 0,
							Hook:          "failure",
						},
						Task: &atc.TaskPlan{
							Name: "some other failure",
						},
					},
				},
			}

			Ω(actual).Should(Equal(expected))
		})
	})

	Context("when I have conditionals and hooks in my plan", func() {
		It("errors", func() {
			_, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "those who resist our will",
						Failure: &atc.PlanConfig{
							Task: "those who did not resist our will",
						},
					},
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some other thing",
							},
							{
								Task:       "some other failure",
								Conditions: &atc.Conditions{atc.ConditionFailure},
							},
						},
					},
				},
			}, resources, nil)
			Ω(err).Should(HaveOccurred())

			_, err = buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "those who resist our will",
						Failure: &atc.PlanConfig{
							Task: "those who did not resist our will",
						},
					},
					{
						Do: &atc.PlanSequence{
							{
								Task: "some other thing",
							},
							{
								Task:       "some other failure",
								Conditions: &atc.Conditions{atc.ConditionFailure},
							},
						},
					},
				},
			}, resources, nil)
			Ω(err).Should(HaveOccurred())

			_, err = buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "those who resist our will",
					},
					{
						Do: &atc.PlanSequence{
							{
								Task: "some other thing",
							},
							{
								Task: "some other thing",
								Failure: &atc.PlanConfig{
									Task: "those who did not resist our will",
								},
							},
							{
								Task:       "some other failure",
								Conditions: &atc.Conditions{atc.ConditionFailure},
							},
						},
					},
				},
			}, resources, nil)

			Ω(err).Should(HaveOccurred())

			_, err = buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "those who resist our will",
					},
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some other thing",
							},
							{
								Task: "some other thing",
								Failure: &atc.PlanConfig{
									Task: "those who did not resist our will",
								},
							},
							{
								Task:       "some other failure",
								Conditions: &atc.Conditions{atc.ConditionFailure},
							},
						},
					},
				},
			}, resources, nil)
			Ω(err).Should(HaveOccurred())

			_, err = buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "those who resist our will",
					},
					{
						Do: &atc.PlanSequence{
							{
								Task: "some other thing",
							},
							{
								Task: "some other thing",
								Ensure: &atc.PlanConfig{
									Task: "those who did not resist our will",
								},
							},
							{
								Task:       "some other failure",
								Conditions: &atc.Conditions{atc.ConditionFailure},
							},
						},
					},
				},
			}, resources, nil)

			Ω(err).Should(HaveOccurred())

			_, err = buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "those who resist our will",
					},
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some other thing",
							},
							{
								Task: "some other thing",
								Ensure: &atc.PlanConfig{
									Task: "those who did not resist our will",
								},
							},
							{
								Task:       "some other failure",
								Conditions: &atc.Conditions{atc.ConditionFailure},
							},
						},
					},
				},
			}, resources, nil)
			Ω(err).Should(HaveOccurred())

			_, err = buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "those who resist our will",
					},
					{
						Do: &atc.PlanSequence{
							{
								Task: "some other thing",
							},
							{
								Task: "some other thing",
								Success: &atc.PlanConfig{
									Task: "those who did not resist our will",
								},
							},
							{
								Task:       "some other failure",
								Conditions: &atc.Conditions{atc.ConditionFailure},
							},
						},
					},
				},
			}, resources, nil)

			Ω(err).Should(HaveOccurred())

			_, err = buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "those who resist our will",
					},
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some other thing",
							},
							{
								Task: "some other thing",
								Success: &atc.PlanConfig{
									Task: "those who did not resist our will",
								},
							},
							{
								Task:       "some other failure",
								Conditions: &atc.Conditions{atc.ConditionFailure},
							},
						},
					},
				},
			}, resources, nil)
			Ω(err).Should(HaveOccurred())
		})
	})

	Context("when I have an empty plan", func() {
		It("returns an empty plan", func() {
			actual, err := buildFactory.Create(atc.JobConfig{}, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{}
			Ω(actual).Should(Equal(expected))
		})
	})

	Context("when I have aggregate in an aggregate in a hook", func() {
		var input atc.JobConfig

		BeforeEach(func() {
			input = atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "some-task",
						Success: &atc.PlanConfig{
							Aggregate: &atc.PlanSequence{
								{
									Task: "agg-task-1",
								},
								{
									Aggregate: &atc.PlanSequence{
										{
											Task: "agg-agg-task-1",
										},
									},
								},
							},
						},
					},
				},
			}
		})

		It("builds correctly", func() {
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
						Aggregate: &atc.AggregatePlan{
							{
								Location: &atc.Location{
									ParentID:      1,
									ID:            4,
									ParallelGroup: 3,
									Hook:          "success",
								},
								Task: &atc.TaskPlan{
									Name: "agg-task-1",
								},
							},
							{
								Aggregate: &atc.AggregatePlan{
									{
										Location: &atc.Location{
											ParentID:      3,
											ID:            7,
											ParallelGroup: 6,
										},
										Task: &atc.TaskPlan{
											Name: "agg-agg-task-1",
										},
									},
								},
							},
						},
					},
				},
			}

			Ω(actual).To(Equal(expected))
		})
	})

	Context("when I have nested do in a hook", func() {
		var input atc.JobConfig

		BeforeEach(func() {
			input = atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "some-task",
						Success: &atc.PlanConfig{
							Do: &atc.PlanSequence{
								{
									Task: "do-task-1",
								},
							},
						},
					},
				},
			}
		})

		It("builds correctly", func() {
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
						Location: &atc.Location{
							ParentID:      1,
							ID:            3,
							ParallelGroup: 0,
							Hook:          "success",
						},
						Task: &atc.TaskPlan{
							Name: "do-task-1",
						},
					},
				},
			}

			Ω(actual).To(Equal(expected))
		})
	})

	Context("when I have nested aggregates in a hook", func() {
		var input atc.JobConfig

		BeforeEach(func() {
			input = atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "some-task",
						Success: &atc.PlanConfig{
							Aggregate: &atc.PlanSequence{
								{
									Task: "agg-task-1",
									Success: &atc.PlanConfig{
										Task: "agg-task-1-success",
									},
								},
								{
									Task: "agg-task-2",
								},
							},
						},
					},
				},
			}
		})

		It("builds correctly", func() {
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
						Aggregate: &atc.AggregatePlan{
							{
								OnSuccess: &atc.OnSuccessPlan{
									Step: atc.Plan{
										Location: &atc.Location{
											ParentID:      1,
											ID:            4,
											ParallelGroup: 3,
											Hook:          "success",
										},
										Task: &atc.TaskPlan{
											Name: "agg-task-1",
										},
									},
									Next: atc.Plan{
										Location: &atc.Location{
											ParentID:      4,
											ID:            5,
											ParallelGroup: 0,
											Hook:          "success",
										},
										Task: &atc.TaskPlan{
											Name: "agg-task-1-success",
										},
									},
								},
							},
							{
								Location: &atc.Location{
									ParentID:      1,
									ID:            6,
									ParallelGroup: 3,
									Hook:          "success",
								},
								Task: &atc.TaskPlan{
									Name: "agg-task-2",
								},
							},
						},
					},
				},
			}

			Ω(actual).To(Equal(expected))
		})
	})

	// Context("when I have conditionals in my plan", func() {
	// 	It("builds a plan with conditionals", func() {
	// 		actual, err := buildFactory.Create(atc.JobConfig{
	// 			Plan: atc.PlanSequence{
	// 				{
	// 					Task: "those who resist our will",
	// 				},
	// 				{
	// 					Task:       "some other failure",
	// 					Conditions: &atc.Conditions{atc.ConditionFailure},
	// 				},
	// 			},
	// 		}, resources, nil)
	// 		Ω(err).ShouldNot(HaveOccurred())

	// 		expected := atc.Plan{
	// 			Compose: &atc.ComposePlan{
	// 				A: atc.Plan{
	// 					Task: &atc.TaskPlan{
	// 						Name: "those who resist our will",
	// 					},
	// 				},
	// 				B: atc.Plan{
	// 					Conditional: &atc.ConditionalPlan{
	// 						Conditions: atc.Conditions{atc.ConditionFailure},
	// 						Plan: atc.Plan{
	// 							Task: &atc.TaskPlan{
	// 								Name: "some other failure",
	// 							},
	// 						},
	// 					},
	// 				},
	// 			},
	// 		}
	// 		Ω(actual).Should(Equal(expected))
	// 	})
	// })

	Context("when I have hooks in my plan", func() {
		It("can build a job with one failure hook", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "those who resist our will",
						Failure: &atc.PlanConfig{
							Get: "some-resource",
						},
					},
				},
			}, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				OnFailure: &atc.OnFailurePlan{
					Step: atc.Plan{
						Task: &atc.TaskPlan{
							Name: "those who resist our will",
						},
						Location: &atc.Location{
							ID:            1,
							ParallelGroup: 0,
							ParentID:      0,
							Hook:          "",
						},
					},
					Next: atc.Plan{
						Get: &atc.GetPlan{
							Name:     "some-resource",
							Type:     "git",
							Resource: "some-resource",
							Pipeline: "some-pipeline",
							Source: atc.Source{
								"uri": "git://some-resource",
							},
						},
						Location: &atc.Location{
							ID:            2,
							ParallelGroup: 0,
							ParentID:      1,
							Hook:          "failure",
						},
					},
				},
			}

			Ω(actual).Should(Equal(expected))

		})

		It("can build a job with one failure hook that has a timeout", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "those who resist our will",
						Failure: &atc.PlanConfig{
							Get:     "some-resource",
							Timeout: "10s",
						},
					},
				},
			}, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				OnFailure: &atc.OnFailurePlan{
					Step: atc.Plan{
						Location: &atc.Location{
							ParentID: 0,
							ID:       1,
							Hook:     "",
						},
						Task: &atc.TaskPlan{
							Name: "those who resist our will",
						},
					},
					Next: atc.Plan{
						Timeout: &atc.TimeoutPlan{
							Duration: "10s",
							Step: atc.Plan{
								Location: &atc.Location{
									ParentID: 1,
									ID:       2,
									Hook:     "failure",
								},
								Get: &atc.GetPlan{
									Name:     "some-resource",
									Type:     "git",
									Resource: "some-resource",
									Pipeline: "some-pipeline",
									Source: atc.Source{
										"uri": "git://some-resource",
									},
								},
							},
						},
					},
				},
			}

			Ω(actual).Should(Equal(expected))
		})

		It("can build a job with multiple failure hooks", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "those who resist our will",
						Failure: &atc.PlanConfig{
							Get: "some-resource",
							Failure: &atc.PlanConfig{
								Task: "those who still resist our will",
							},
						},
					},
				},
			}, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				OnFailure: &atc.OnFailurePlan{
					Step: atc.Plan{
						Location: &atc.Location{
							ID:            1,
							ParentID:      0,
							ParallelGroup: 0,
						},
						Task: &atc.TaskPlan{
							Name: "those who resist our will",
						},
					},
					Next: atc.Plan{
						OnFailure: &atc.OnFailurePlan{
							Step: atc.Plan{
								Location: &atc.Location{
									ID:            2,
									ParentID:      1,
									ParallelGroup: 0,
									Hook:          "failure",
								},
								Get: &atc.GetPlan{
									Name:     "some-resource",
									Type:     "git",
									Resource: "some-resource",
									Pipeline: "some-pipeline",
									Source: atc.Source{
										"uri": "git://some-resource",
									},
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{
									ID:            3,
									ParentID:      2,
									ParallelGroup: 0,
									Hook:          "failure",
								},
								Task: &atc.TaskPlan{
									Name: "those who still resist our will",
								},
							},
						},
					},
				},
			}

			Ω(actual).Should(Equal(expected))
		})

		It("can build a job with multiple ensure and failure hooks", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "those who resist our will",
						Failure: &atc.PlanConfig{
							Get: "some-resource",
							Ensure: &atc.PlanConfig{
								Task: "those who still resist our will",
							},
						},
					},
				},
			}, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				OnFailure: &atc.OnFailurePlan{
					Step: atc.Plan{
						Location: &atc.Location{
							ID:            1,
							ParentID:      0,
							ParallelGroup: 0,
						},
						Task: &atc.TaskPlan{
							Name: "those who resist our will",
						},
					},
					Next: atc.Plan{
						Ensure: &atc.EnsurePlan{
							Step: atc.Plan{
								Location: &atc.Location{
									ID:            2,
									ParentID:      1,
									ParallelGroup: 0,
									Hook:          "failure",
								},
								Get: &atc.GetPlan{
									Name:     "some-resource",
									Type:     "git",
									Resource: "some-resource",
									Pipeline: "some-pipeline",
									Source: atc.Source{
										"uri": "git://some-resource",
									},
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{
									ID:            3,
									ParentID:      2,
									ParallelGroup: 0,
									Hook:          "ensure",
								},
								Task: &atc.TaskPlan{
									Name: "those who still resist our will",
								},
							},
						},
					},
				},
			}

			Ω(actual).Should(Equal(expected))
		})

		It("can build a job with failure, success and ensure hooks at the same level", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "those who resist our will",
						Failure: &atc.PlanConfig{
							Task: "those who failed to resist our will",
						},
						Ensure: &atc.PlanConfig{
							Task: "those who always resist our will",
						},
						Success: &atc.PlanConfig{
							Task: "those who successfully resisted our will",
						},
					},
				},
			}, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				Ensure: &atc.EnsurePlan{
					Step: atc.Plan{
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								OnFailure: &atc.OnFailurePlan{
									Step: atc.Plan{
										Location: &atc.Location{
											ID:            1,
											ParentID:      0,
											ParallelGroup: 0,
										},
										Task: &atc.TaskPlan{
											Name: "those who resist our will",
										},
									},
									Next: atc.Plan{
										Location: &atc.Location{
											ID:            2,
											ParentID:      1,
											ParallelGroup: 0,
											Hook:          "failure",
										},
										Task: &atc.TaskPlan{
											Name: "those who failed to resist our will",
										},
									},
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{
									ID:            3,
									ParentID:      1,
									ParallelGroup: 0,
									Hook:          "success",
								},
								Task: &atc.TaskPlan{
									Name: "those who successfully resisted our will",
								},
							},
						},
					},
					Next: atc.Plan{
						Location: &atc.Location{
							ID:            4,
							ParentID:      1,
							ParallelGroup: 0,
							Hook:          "ensure",
						},
						Task: &atc.TaskPlan{
							Name: "those who always resist our will",
						},
					},
				},
			}

			Ω(actual).Should(Equal(expected))
		})

		It("can build a job with multiple ensure, failure and success hooks", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "those who resist our will",
						Failure: &atc.PlanConfig{
							Get: "some-resource",
							Ensure: &atc.PlanConfig{
								Task: "those who still resist our will",
							},
						},
						Success: &atc.PlanConfig{
							Get: "some-resource",
						},
					},
				},
			}, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				OnSuccess: &atc.OnSuccessPlan{
					Step: atc.Plan{
						OnFailure: &atc.OnFailurePlan{
							Step: atc.Plan{
								Location: &atc.Location{
									ID:            1,
									ParentID:      0,
									ParallelGroup: 0,
								},
								Task: &atc.TaskPlan{
									Name: "those who resist our will",
								},
							},
							Next: atc.Plan{
								Ensure: &atc.EnsurePlan{
									Step: atc.Plan{
										Location: &atc.Location{
											ID:            2,
											ParentID:      1,
											ParallelGroup: 0,
											Hook:          "failure",
										},
										Get: &atc.GetPlan{
											Name:     "some-resource",
											Type:     "git",
											Resource: "some-resource",
											Pipeline: "some-pipeline",
											Source: atc.Source{
												"uri": "git://some-resource",
											},
										},
									},
									Next: atc.Plan{
										Location: &atc.Location{
											ID:            3,
											ParentID:      2,
											ParallelGroup: 0,
											Hook:          "ensure",
										},
										Task: &atc.TaskPlan{
											Name: "those who still resist our will",
										},
									},
								},
							},
						},
					},
					Next: atc.Plan{
						Location: &atc.Location{
							ID:            4,
							ParentID:      1,
							ParallelGroup: 0,
							Hook:          "success",
						},
						Get: &atc.GetPlan{
							Name:     "some-resource",
							Type:     "git",
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

		Context("and multiple steps in my plan", func() {
			It("can build a job with a task with hooks then 2 more tasks", func() {
				actual, err := buildFactory.Create(atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Task: "those who resist our will",
							Failure: &atc.PlanConfig{
								Task: "some other task",
							},
							Success: &atc.PlanConfig{
								Task: "some other success task",
							},
						},
						{
							Task: "those who still resist our will",
						},
						{
							Task: "shall be defeated",
						},
					},
				}, resources, nil)
				Ω(err).ShouldNot(HaveOccurred())

				expected := atc.Plan{
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									OnFailure: &atc.OnFailurePlan{
										Step: atc.Plan{
											Location: &atc.Location{
												ID:            1,
												ParentID:      0,
												ParallelGroup: 0,
											},
											Task: &atc.TaskPlan{
												Name: "those who resist our will",
											},
										},
										Next: atc.Plan{
											Location: &atc.Location{
												ID:            2,
												ParentID:      1,
												ParallelGroup: 0,
												Hook:          "failure",
											},
											Task: &atc.TaskPlan{
												Name: "some other task",
											},
										},
									},
								},
								Next: atc.Plan{
									Location: &atc.Location{
										ID:            3,
										ParentID:      1,
										ParallelGroup: 0,
										Hook:          "success",
									},
									Task: &atc.TaskPlan{
										Name: "some other success task",
									},
								},
							},
						},
						Next: atc.Plan{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									Location: &atc.Location{
										ID:            4,
										ParentID:      0,
										ParallelGroup: 0,
										Hook:          "",
									},
									Task: &atc.TaskPlan{
										Name: "those who still resist our will",
									},
								},
								Next: atc.Plan{
									Location: &atc.Location{
										ID:            5,
										ParentID:      0,
										ParallelGroup: 0,
										Hook:          "",
									},
									Task: &atc.TaskPlan{
										Name: "shall be defeated",
									},
								},
							},
						},
					},
				}
				Ω(actual).Should(Equal(expected))
			})

			It("can build a job with a task then a do", func() {
				actual, err := buildFactory.Create(atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Task: "those who start resisting our will",
						},
						{
							Do: &atc.PlanSequence{
								{
									Task: "those who resist our will",
									Failure: &atc.PlanConfig{
										Task: "some other task",
									},
									Success: &atc.PlanConfig{
										Task: "some other success task",
									},
								},
								{
									Task: "those who used to resist our will",
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
								ID:            1,
								ParentID:      0,
								ParallelGroup: 0,
							},
							Task: &atc.TaskPlan{
								Name: "those who start resisting our will",
							},
						},
						Next: atc.Plan{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									OnSuccess: &atc.OnSuccessPlan{
										Step: atc.Plan{
											OnFailure: &atc.OnFailurePlan{
												Step: atc.Plan{
													Location: &atc.Location{
														ID:            3,
														ParentID:      0,
														ParallelGroup: 0,
													},
													Task: &atc.TaskPlan{
														Name: "those who resist our will",
													},
												},
												Next: atc.Plan{
													Location: &atc.Location{
														ID:            4,
														ParentID:      3,
														ParallelGroup: 0,
														Hook:          "failure",
													},
													Task: &atc.TaskPlan{
														Name: "some other task",
													},
												},
											},
										},
										Next: atc.Plan{
											Location: &atc.Location{
												ID:            5,
												ParentID:      3,
												ParallelGroup: 0,
												Hook:          "success",
											},
											Task: &atc.TaskPlan{
												Name: "some other success task",
											},
										},
									},
								},
								Next: atc.Plan{
									Location: &atc.Location{
										ID:            6,
										ParentID:      0,
										ParallelGroup: 0,
									},
									Task: &atc.TaskPlan{
										Name: "those who used to resist our will",
									},
								},
							},
						},
					},
				}
				Ω(actual).Should(Equal(expected))
			})

			It("can build a job with a do then a task", func() {
				actual, err := buildFactory.Create(atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Do: &atc.PlanSequence{
								{
									Task: "those who resist our will",
								},
								{
									Task: "those who used to resist our will",
								},
							},
						},
						{
							Task: "those who start resisting our will",
						},
					},
				}, resources, nil)
				Ω(err).ShouldNot(HaveOccurred())

				expected := atc.Plan{
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									Location: &atc.Location{
										ID:            2,
										ParentID:      0,
										ParallelGroup: 0,
									},
									Task: &atc.TaskPlan{
										Name: "those who resist our will",
									},
								},
								Next: atc.Plan{
									Location: &atc.Location{
										ID:            3,
										ParentID:      0,
										ParallelGroup: 0,
									},
									Task: &atc.TaskPlan{
										Name: "those who used to resist our will",
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
								Name: "those who start resisting our will",
							},
						},
					},
				}

				Ω(actual).Should(Equal(expected))
			})
		})
	})
})
