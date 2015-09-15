package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"
	"github.com/concourse/atc/scheduler/factory/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Hooks", func() {
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

	Context("when there is a do with three steps with a hook", func() {
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
							{
								Task: "third task",
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
								Task: &atc.TaskPlan{
									Name:     "those who resist our will",
									Pipeline: "some-pipeline",
								},
							},
							Next: atc.Plan{
								OnSuccess: &atc.OnSuccessPlan{
									Step: atc.Plan{
										Task: &atc.TaskPlan{
											Name:     "those who also resist our will",
											Pipeline: "some-pipeline",
										},
									},
									Next: atc.Plan{
										Task: &atc.TaskPlan{
											Name:     "third task",
											Pipeline: "some-pipeline",
										},
									},
								},
							},
						},
					},
					Next: atc.Plan{
						Task: &atc.TaskPlan{
							Name:     "some other failure",
							Pipeline: "some-pipeline",
						},
					},
				},
			}

			Ω(actual).Should(Equal(expected))
		})
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
								Task: &atc.TaskPlan{
									Name:     "those who resist our will",
									Pipeline: "some-pipeline",
								},
							},
							Next: atc.Plan{
								Task: &atc.TaskPlan{
									Name:     "those who also resist our will",
									Pipeline: "some-pipeline",
								},
							},
						},
					},
					Next: atc.Plan{
						Task: &atc.TaskPlan{
							Name:     "some other failure",
							Pipeline: "some-pipeline",
						},
					},
				},
			}

			Ω(actual).Should(Equal(expected))
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
						Task: &atc.TaskPlan{
							Name:     "some-task",
							Pipeline: "some-pipeline",
						},
					},
					Next: atc.Plan{
						Aggregate: &atc.AggregatePlan{
							{
								Task: &atc.TaskPlan{
									Name:     "agg-task-1",
									Pipeline: "some-pipeline",
								},
							},
							{
								Aggregate: &atc.AggregatePlan{
									{
										Task: &atc.TaskPlan{
											Name:     "agg-agg-task-1",
											Pipeline: "some-pipeline",
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
						Task: &atc.TaskPlan{
							Name:     "some-task",
							Pipeline: "some-pipeline",
						},
					},
					Next: atc.Plan{
						Task: &atc.TaskPlan{
							Name:     "do-task-1",
							Pipeline: "some-pipeline",
						},
					},
				},
			}

			Ω(actual).To(Equal(expected))
		})
	})

	Context("when I have multiple nested do steps in hooks", func() {
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
								{
									Do: &atc.PlanSequence{
										{
											Task: "do-task-2",
										},
										{
											Task: "do-task-3",
											Success: &atc.PlanConfig{
												Task: "do-task-4",
											},
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
						Task: &atc.TaskPlan{
							Name:     "some-task",
							Pipeline: "some-pipeline",
						},
					},
					Next: atc.Plan{
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								Task: &atc.TaskPlan{
									Name:     "do-task-1",
									Pipeline: "some-pipeline",
								},
							},
							Next: atc.Plan{
								OnSuccess: &atc.OnSuccessPlan{
									Step: atc.Plan{
										Task: &atc.TaskPlan{
											Name:     "do-task-2",
											Pipeline: "some-pipeline",
										},
									},
									Next: atc.Plan{
										OnSuccess: &atc.OnSuccessPlan{
											Step: atc.Plan{
												Task: &atc.TaskPlan{
													Name:     "do-task-3",
													Pipeline: "some-pipeline",
												},
											},
											Next: atc.Plan{
												Task: &atc.TaskPlan{
													Name:     "do-task-4",
													Pipeline: "some-pipeline",
												},
											},
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
						Task: &atc.TaskPlan{
							Name:     "some-task",
							Pipeline: "some-pipeline",
						},
					},
					Next: atc.Plan{
						Aggregate: &atc.AggregatePlan{
							{
								OnSuccess: &atc.OnSuccessPlan{
									Step: atc.Plan{
										Task: &atc.TaskPlan{
											Name:     "agg-task-1",
											Pipeline: "some-pipeline",
										},
									},
									Next: atc.Plan{
										Task: &atc.TaskPlan{
											Name:     "agg-task-1-success",
											Pipeline: "some-pipeline",
										},
									},
								},
							},
							{
								Task: &atc.TaskPlan{
									Name:     "agg-task-2",
									Pipeline: "some-pipeline",
								},
							},
						},
					},
				},
			}

			Ω(actual).To(Equal(expected))
		})
	})

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
							Name:     "those who resist our will",
							Pipeline: "some-pipeline",
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
						Task: &atc.TaskPlan{
							Name:     "those who resist our will",
							Pipeline: "some-pipeline",
						},
					},
					Next: atc.Plan{
						Timeout: &atc.TimeoutPlan{
							Duration: "10s",
							Step: atc.Plan{
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
						Task: &atc.TaskPlan{
							Name:     "those who resist our will",
							Pipeline: "some-pipeline",
						},
					},
					Next: atc.Plan{
						OnFailure: &atc.OnFailurePlan{
							Step: atc.Plan{
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
								Task: &atc.TaskPlan{
									Name:     "those who still resist our will",
									Pipeline: "some-pipeline",
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
						Task: &atc.TaskPlan{
							Name:     "those who resist our will",
							Pipeline: "some-pipeline",
						},
					},
					Next: atc.Plan{
						Ensure: &atc.EnsurePlan{
							Step: atc.Plan{
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
								Task: &atc.TaskPlan{
									Name:     "those who still resist our will",
									Pipeline: "some-pipeline",
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
										Task: &atc.TaskPlan{
											Name:     "those who resist our will",
											Pipeline: "some-pipeline",
										},
									},
									Next: atc.Plan{
										Task: &atc.TaskPlan{
											Name:     "those who failed to resist our will",
											Pipeline: "some-pipeline",
										},
									},
								},
							},
							Next: atc.Plan{
								Task: &atc.TaskPlan{
									Name:     "those who successfully resisted our will",
									Pipeline: "some-pipeline",
								},
							},
						},
					},
					Next: atc.Plan{
						Task: &atc.TaskPlan{
							Name:     "those who always resist our will",
							Pipeline: "some-pipeline",
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
								Task: &atc.TaskPlan{
									Name:     "those who resist our will",
									Pipeline: "some-pipeline",
								},
							},
							Next: atc.Plan{
								Ensure: &atc.EnsurePlan{
									Step: atc.Plan{
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
										Task: &atc.TaskPlan{
											Name:     "those who still resist our will",
											Pipeline: "some-pipeline",
										},
									},
								},
							},
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
											Task: &atc.TaskPlan{
												Name:     "those who resist our will",
												Pipeline: "some-pipeline",
											},
										},
										Next: atc.Plan{
											Task: &atc.TaskPlan{
												Name:     "some other task",
												Pipeline: "some-pipeline",
											},
										},
									},
								},
								Next: atc.Plan{
									Task: &atc.TaskPlan{
										Name:     "some other success task",
										Pipeline: "some-pipeline",
									},
								},
							},
						},
						Next: atc.Plan{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									Task: &atc.TaskPlan{
										Name:     "those who still resist our will",
										Pipeline: "some-pipeline",
									},
								},
								Next: atc.Plan{
									Task: &atc.TaskPlan{
										Name:     "shall be defeated",
										Pipeline: "some-pipeline",
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
							Task: &atc.TaskPlan{
								Name:     "those who start resisting our will",
								Pipeline: "some-pipeline",
							},
						},
						Next: atc.Plan{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									OnSuccess: &atc.OnSuccessPlan{
										Step: atc.Plan{
											OnFailure: &atc.OnFailurePlan{
												Step: atc.Plan{
													Task: &atc.TaskPlan{
														Name:     "those who resist our will",
														Pipeline: "some-pipeline",
													},
												},
												Next: atc.Plan{
													Task: &atc.TaskPlan{
														Name:     "some other task",
														Pipeline: "some-pipeline",
													},
												},
											},
										},
										Next: atc.Plan{
											Task: &atc.TaskPlan{
												Name:     "some other success task",
												Pipeline: "some-pipeline",
											},
										},
									},
								},
								Next: atc.Plan{
									Task: &atc.TaskPlan{
										Name:     "those who used to resist our will",
										Pipeline: "some-pipeline",
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
									Task: &atc.TaskPlan{
										Name:     "those who resist our will",
										Pipeline: "some-pipeline",
									},
								},
								Next: atc.Plan{
									Task: &atc.TaskPlan{
										Name:     "those who used to resist our will",
										Pipeline: "some-pipeline",
									},
								},
							},
						},

						Next: atc.Plan{
							Task: &atc.TaskPlan{
								Name:     "those who start resisting our will",
								Pipeline: "some-pipeline",
							},
						},
					},
				}

				Ω(actual).Should(Equal(expected))
			})
		})
	})
})
