package factory_test

import (
	"time"

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

	Context("when I have conditionals in my plan", func() {
		It("builds a plan with conditionals", func() {
			actual, err := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "those who resist our will",
					},
					{
						Task:       "some other failure",
						Conditions: &atc.Conditions{atc.ConditionFailure},
					},
				},
			}, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				Compose: &atc.ComposePlan{
					A: atc.Plan{
						Task: &atc.TaskPlan{
							Name: "those who resist our will",
						},
					},
					B: atc.Plan{
						Conditional: &atc.ConditionalPlan{
							Conditions: atc.Conditions{atc.ConditionFailure},
							Plan: atc.Plan{
								Task: &atc.TaskPlan{
									Name: "some other failure",
								},
							},
						},
					},
				},
			}
			Ω(actual).Should(Equal(expected))
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
				HookedCompose: &atc.HookedComposePlan{
					Step: atc.Plan{
						Task: &atc.TaskPlan{
							Name: "those who resist our will",
						},
					},
					OnFailure: atc.Plan{
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
							Timeout: atc.Duration(10 * time.Second),
						},
					},
				},
			}, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			expected := atc.Plan{
				HookedCompose: &atc.HookedComposePlan{
					Step: atc.Plan{
						Task: &atc.TaskPlan{
							Name: "those who resist our will",
						},
					},
					OnFailure: atc.Plan{
						Timeout: &atc.TimeoutPlan{
							Duration: atc.Duration(10 * time.Second),
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
				HookedCompose: &atc.HookedComposePlan{
					Step: atc.Plan{
						Task: &atc.TaskPlan{
							Name: "those who resist our will",
						},
					},
					OnFailure: atc.Plan{
						HookedCompose: &atc.HookedComposePlan{
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
							OnFailure: atc.Plan{
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
				HookedCompose: &atc.HookedComposePlan{
					Step: atc.Plan{
						Task: &atc.TaskPlan{
							Name: "those who resist our will",
						},
					},
					OnFailure: atc.Plan{
						HookedCompose: &atc.HookedComposePlan{
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
							OnCompletion: atc.Plan{
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
				HookedCompose: &atc.HookedComposePlan{
					Step: atc.Plan{
						Task: &atc.TaskPlan{
							Name: "those who resist our will",
						},
					},
					OnSuccess: atc.Plan{
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
					OnFailure: atc.Plan{
						HookedCompose: &atc.HookedComposePlan{
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
							OnCompletion: atc.Plan{
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

		Context("and multiple steps in my plan", func() {
			It("can build a job with a task with hooks then 2 mores tasks", func() {
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
					HookedCompose: &atc.HookedComposePlan{
						Step: atc.Plan{
							Task: &atc.TaskPlan{
								Name: "those who resist our will",
							},
						},
						OnFailure: atc.Plan{
							Task: &atc.TaskPlan{
								Name: "some other task",
							},
						},
						OnSuccess: atc.Plan{
							Task: &atc.TaskPlan{
								Name: "some other success task",
							},
						},
						Next: atc.Plan{
							HookedCompose: &atc.HookedComposePlan{
								Step: atc.Plan{
									Task: &atc.TaskPlan{
										Name: "those who still resist our will",
									},
								},
								Next: atc.Plan{
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
					HookedCompose: &atc.HookedComposePlan{
						Step: atc.Plan{
							Task: &atc.TaskPlan{
								Name: "those who start resisting our will",
							},
						},
						Next: atc.Plan{
							HookedCompose: &atc.HookedComposePlan{
								Step: atc.Plan{
									Task: &atc.TaskPlan{
										Name: "those who resist our will",
									},
								},
								OnFailure: atc.Plan{
									Task: &atc.TaskPlan{
										Name: "some other task",
									},
								},
								OnSuccess: atc.Plan{
									Task: &atc.TaskPlan{
										Name: "some other success task",
									},
								},
								Next: atc.Plan{
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
					HookedCompose: &atc.HookedComposePlan{
						Step: atc.Plan{
							Task: &atc.TaskPlan{
								Name: "those who start resisting our will",
							},
						},
						Next: atc.Plan{
							HookedCompose: &atc.HookedComposePlan{
								Step: atc.Plan{
									Task: &atc.TaskPlan{
										Name: "those who resist our will",
									},
								},
								OnFailure: atc.Plan{
									Task: &atc.TaskPlan{
										Name: "some other task",
									},
								},
								OnSuccess: atc.Plan{
									Task: &atc.TaskPlan{
										Name: "some other success task",
									},
								},
								Next: atc.Plan{
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
					HookedCompose: &atc.HookedComposePlan{
						Step: atc.Plan{
							HookedCompose: &atc.HookedComposePlan{
								Step: atc.Plan{
									Task: &atc.TaskPlan{
										Name: "those who resist our will",
									},
								},
								Next: atc.Plan{
									Task: &atc.TaskPlan{
										Name: "those who used to resist our will",
									},
								},
							},
						},
						Next: atc.Plan{
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
