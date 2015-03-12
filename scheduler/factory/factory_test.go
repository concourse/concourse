package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/scheduler/factory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory", func() {
	var (
		factory *BuildFactory

		job       atc.JobConfig
		resources atc.ResourceConfigs

		expectedPlan atc.Plan
	)

	BeforeEach(func() {
		factory = &BuildFactory{}

		job = atc.JobConfig{
			Name: "some-job",
		}

		expectedPlan = atc.Plan{
			Compose: &atc.ComposePlan{
				A: atc.Plan{
					Aggregate: &atc.AggregatePlan{
						"some-input": atc.Plan{
							Get: &atc.GetPlan{
								Type:     "git",
								Name:     "some-input",
								Resource: "some-resource",
								Source:   atc.Source{"uri": "git://some-resource"},
								Params:   atc.Params{"some": "params"},
							},
						},
					},
				},
				B: atc.Plan{
					Compose: &atc.ComposePlan{
						A: atc.Plan{
							Task: &atc.TaskPlan{
								Name: "build",

								Privileged: true,

								ConfigPath: "some-input/build.yml",
								Config: &atc.TaskConfig{
									Image: "some-image",

									Params: map[string]string{
										"FOO": "1",
										"BAR": "2",
									},

									Run: atc.TaskRunConfig{
										Path: "some-script",
										Args: []string{"arg1", "arg2"},
									},
								},
							},
						},
						B: atc.Plan{
							Aggregate: &atc.AggregatePlan{
								"some-resource": atc.Plan{
									Conditional: &atc.ConditionalPlan{
										Conditions: []atc.Condition{atc.ConditionSuccess},
										Plan: atc.Plan{
											Put: &atc.PutPlan{
												Name:     "some-resource",
												Resource: "some-resource",
												Type:     "git",
												Params:   atc.Params{"foo": "bar"},
												Source:   atc.Source{"uri": "git://some-resource"},
											},
										},
									},
								},
								"some-other-resource": atc.Plan{
									Conditional: &atc.ConditionalPlan{
										Conditions: []atc.Condition{atc.ConditionFailure},
										Plan: atc.Plan{
											Put: &atc.PutPlan{
												Name:     "some-other-resource",
												Resource: "some-other-resource",
												Type:     "git",
												Params:   atc.Params{"foo": "bar"},
												Source:   atc.Source{"uri": "git://some-other-resource"},
											},
										},
									},
								},
								"some-other-other-resource": atc.Plan{
									Conditional: &atc.ConditionalPlan{
										Conditions: []atc.Condition{},
										Plan: atc.Plan{
											Put: &atc.PutPlan{
												Name:     "some-other-other-resource",
												Resource: "some-other-other-resource",
												Type:     "git",
												Params:   atc.Params{"foo": "bar"},
												Source:   atc.Source{"uri": "git://some-other-other-resource"},
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

		resources = atc.ResourceConfigs{
			{
				Name:   "some-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-resource"},
			},
			{
				Name:   "some-other-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-other-resource"},
			},
			{
				Name:   "some-other-other-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-other-other-resource"},
			},
			{
				Name:   "some-dependant-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-dependant-resource"},
			},
			{
				Name:   "some-output-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-output-resource"},
			},
			{
				Name:   "some-resource-with-longer-name",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-resource-with-longer-name"},
			},
			{
				Name:   "some-named-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-named-resource"},
			},
		}
	})

	Context("when the job has no plan and no inputs or outputs", func() {
		It("returns an empty plan", func() {
			Ω(factory.Create(job, resources, nil)).Should(Equal(atc.Plan{}))
		})
	})

	Context("when the job has a plan", func() { // to take over the world
		BeforeEach(func() {
			job.Plan = atc.PlanSequence{
				{
					Aggregate: &atc.PlanSequence{
						{
							Get:      "some-input",
							Resource: "some-resource",
							Params:   atc.Params{"some": "params"},
						},
					},
				},
				{
					Task:           "build",
					Privileged:     true,
					TaskConfigPath: "some-input/build.yml",
					TaskConfig: &atc.TaskConfig{
						Image: "some-image",
						Params: map[string]string{
							"FOO": "1",
							"BAR": "2",
						},
						Run: atc.TaskRunConfig{
							Path: "some-script",
							Args: []string{"arg1", "arg2"},
						},
					},
				},
				{
					Aggregate: &atc.PlanSequence{
						{
							Conditions: &atc.Conditions{"success"},
							RawName:    "some-resource",
							Do: &atc.PlanSequence{
								{
									Put:    "some-resource",
									Params: atc.Params{"foo": "bar"},
								},
							},
						},
						{
							Conditions: &atc.Conditions{"failure"},
							Put:        "some-other-resource",
							Params:     atc.Params{"foo": "bar"},
						},
						{
							Conditions: &atc.Conditions{},
							Put:        "some-other-other-resource",
							Params:     atc.Params{"foo": "bar"},
						},
					},
				},
			}
		})

		It("uses the plan in the job config if present", func() {
			plan, err := factory.Create(job, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(plan).Should(Equal(expectedPlan))
		})

		Describe("chains of conditional plans", func() {
			It("breaks the chain at each condition", func() {
				Ω(factory.Create(atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Task: "those who resist our will",
						},
						{
							Task:       "the deal",
							Conditions: &atc.Conditions{"success"},
						},
						{
							Task:       "the other deal",
							Conditions: &atc.Conditions{"failure"},
						},
					},
				}, resources, nil)).Should(Equal(atc.Plan{
					Compose: &atc.ComposePlan{
						A: atc.Plan{
							Task: &atc.TaskPlan{
								Name: "those who resist our will",
							},
						},
						B: atc.Plan{
							Conditional: &atc.ConditionalPlan{
								Conditions: atc.Conditions{"success"},
								Plan: atc.Plan{
									Compose: &atc.ComposePlan{
										A: atc.Plan{
											Task: &atc.TaskPlan{
												Name: "the deal",
											},
										},
										B: atc.Plan{
											Conditional: &atc.ConditionalPlan{
												Conditions: atc.Conditions{"failure"},
												Plan: atc.Plan{
													Task: &atc.TaskPlan{
														Name: "the other deal",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}))
			})
		})

		Context("when a get plan follows a task plan", func() {
			Context("with no explicit condition", func() {
				It("runs only on success", func() {
					Ω(factory.Create(atc.JobConfig{
						Plan: atc.PlanSequence{
							{
								Task: "those who resist our will",
							},
							{
								Get: "money",
							},
						},
					}, resources, nil)).Should(Equal(atc.Plan{
						Compose: &atc.ComposePlan{
							A: atc.Plan{
								Task: &atc.TaskPlan{
									Name: "those who resist our will",
								},
							},
							B: atc.Plan{
								Conditional: &atc.ConditionalPlan{
									Conditions: atc.Conditions{"success"},
									Plan: atc.Plan{
										Get: &atc.GetPlan{
											Name:     "money",
											Resource: "money",
										},
									},
								},
							},
						},
					}))
				})
			})

			Context("when it has an explicit condition", func() {
				It("runs with the given condition", func() {
					Ω(factory.Create(atc.JobConfig{
						Plan: atc.PlanSequence{
							{
								Task: "those who resist our will",
							},
							{
								Get:        "money",
								Conditions: &atc.Conditions{"failure"},
							},
						},
					}, resources, nil)).Should(Equal(atc.Plan{
						Compose: &atc.ComposePlan{
							A: atc.Plan{
								Task: &atc.TaskPlan{
									Name: "those who resist our will",
								},
							},
							B: atc.Plan{
								Conditional: &atc.ConditionalPlan{
									Conditions: atc.Conditions{"failure"},
									Plan: atc.Plan{
										Get: &atc.GetPlan{
											Name:     "money",
											Resource: "money",
										},
									},
								},
							},
						},
					}))
				})
			})
		})

		Context("when a put plan follows a task plan", func() {
			Context("with no explicit condition", func() {
				It("runs only on success", func() {
					Ω(factory.Create(atc.JobConfig{
						Plan: atc.PlanSequence{
							{
								Task: "those who resist our will",
							},
							{
								Put: "money",
							},
						},
					}, resources, nil)).Should(Equal(atc.Plan{
						Compose: &atc.ComposePlan{
							A: atc.Plan{
								Task: &atc.TaskPlan{
									Name: "those who resist our will",
								},
							},
							B: atc.Plan{
								Conditional: &atc.ConditionalPlan{
									Conditions: atc.Conditions{"success"},
									Plan: atc.Plan{
										Put: &atc.PutPlan{
											Name:     "money",
											Resource: "money",
										},
									},
								},
							},
						},
					}))
				})
			})

			Context("when it has an explicit condition", func() {
				It("runs with the given condition", func() {
					Ω(factory.Create(atc.JobConfig{
						Plan: atc.PlanSequence{
							{
								Task: "those who resist our will",
							},
							{
								Put:        "money",
								Conditions: &atc.Conditions{"failure"},
							},
						},
					}, resources, nil)).Should(Equal(atc.Plan{
						Compose: &atc.ComposePlan{
							A: atc.Plan{
								Task: &atc.TaskPlan{
									Name: "those who resist our will",
								},
							},
							B: atc.Plan{
								Conditional: &atc.ConditionalPlan{
									Conditions: atc.Conditions{"failure"},
									Plan: atc.Plan{
										Put: &atc.PutPlan{
											Name:     "money",
											Resource: "money",
										},
									},
								},
							},
						},
					}))
				})
			})
		})

		Context("when a task plan follows a task plan", func() {
			Context("with no explicit condition", func() {
				It("runs only on success", func() {
					Ω(factory.Create(atc.JobConfig{
						Plan: atc.PlanSequence{
							{
								Task: "those who resist our will",
							},
							{
								Task: "haters",
							},
						},
					}, resources, nil)).Should(Equal(atc.Plan{
						Compose: &atc.ComposePlan{
							A: atc.Plan{
								Task: &atc.TaskPlan{
									Name: "those who resist our will",
								},
							},
							B: atc.Plan{
								Conditional: &atc.ConditionalPlan{
									Conditions: atc.Conditions{"success"},
									Plan: atc.Plan{
										Task: &atc.TaskPlan{
											Name: "haters",
										},
									},
								},
							},
						},
					}))
				})
			})

			Context("when it has an explicit condition", func() {
				It("runs with the given condition", func() {
					Ω(factory.Create(atc.JobConfig{
						Plan: atc.PlanSequence{
							{
								Task: "those who resist our will",
							},
							{
								Task:       "haters",
								Conditions: &atc.Conditions{"failure"},
							},
						},
					}, resources, nil)).Should(Equal(atc.Plan{
						Compose: &atc.ComposePlan{
							A: atc.Plan{
								Task: &atc.TaskPlan{
									Name: "those who resist our will",
								},
							},
							B: atc.Plan{
								Conditional: &atc.ConditionalPlan{
									Conditions: atc.Conditions{"failure"},
									Plan: atc.Plan{
										Task: &atc.TaskPlan{
											Name: "haters",
										},
									},
								},
							},
						},
					}))
				})
			})
		})

		Context("when an aggregate plan follows a task plan", func() {
			Context("and any of its plans have explicit conditions", func() {
				It("runs them with their given condition", func() {
					Ω(factory.Create(atc.JobConfig{
						Plan: atc.PlanSequence{
							{
								Task: "those who resist our will",
							},
							{
								Aggregate: &atc.PlanSequence{
									{
										Put:        "haters",
										Conditions: &atc.Conditions{"failure"},
									},
									{
										Put: "gonna",
									},
									{
										Put:        "hate",
										Conditions: &atc.Conditions{},
									},
								},
							},
						},
					}, resources, nil)).Should(Equal(atc.Plan{
						Compose: &atc.ComposePlan{
							A: atc.Plan{
								Task: &atc.TaskPlan{
									Name: "those who resist our will",
								},
							},
							B: atc.Plan{
								Aggregate: &atc.AggregatePlan{
									"haters": atc.Plan{
										Conditional: &atc.ConditionalPlan{
											Conditions: atc.Conditions{"failure"},
											Plan: atc.Plan{
												Put: &atc.PutPlan{
													Name:     "haters",
													Resource: "haters",
												},
											},
										},
									},
									"gonna": atc.Plan{
										Conditional: &atc.ConditionalPlan{
											Conditions: atc.Conditions{"success"},
											Plan: atc.Plan{
												Put: &atc.PutPlan{
													Name:     "gonna",
													Resource: "gonna",
												},
											},
										},
									},
									"hate": atc.Plan{
										Conditional: &atc.ConditionalPlan{
											Conditions: atc.Conditions{},
											Plan: atc.Plan{
												Put: &atc.PutPlan{
													Name:     "hate",
													Resource: "hate",
												},
											},
										},
									},
								},
							},
						},
					}))
				})
			})
		})

		Context("when a do plan follows a task plan", func() {
			Context("and its first plan has no explicit condition", func() {
				It("runs only on success", func() {
					Ω(factory.Create(atc.JobConfig{
						Plan: atc.PlanSequence{
							{
								Task: "those who resist our will",
							},
							{
								Do: &atc.PlanSequence{
									{
										Put: "haters",
									},
									{
										Put: "gonna",
									},
									{
										Put:        "hate",
										Conditions: &atc.Conditions{},
									},
								},
							},
						},
					}, resources, nil)).Should(Equal(atc.Plan{
						Compose: &atc.ComposePlan{
							A: atc.Plan{
								Task: &atc.TaskPlan{
									Name: "those who resist our will",
								},
							},
							B: atc.Plan{
								Conditional: &atc.ConditionalPlan{
									Conditions: atc.Conditions{"success"},
									Plan: atc.Plan{
										Compose: &atc.ComposePlan{
											A: atc.Plan{
												Put: &atc.PutPlan{
													Name:     "haters",
													Resource: "haters",
												},
											},
											B: atc.Plan{
												Compose: &atc.ComposePlan{
													A: atc.Plan{
														Put: &atc.PutPlan{
															Name:     "gonna",
															Resource: "gonna",
														},
													},
													B: atc.Plan{
														Conditional: &atc.ConditionalPlan{
															Conditions: atc.Conditions{},
															Plan: atc.Plan{
																Put: &atc.PutPlan{
																	Name:     "hate",
																	Resource: "hate",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					}))
				})
			})

			Context("and its first plan has an explicit condition", func() {
				It("runs with the given condition", func() {
					Ω(factory.Create(atc.JobConfig{
						Plan: atc.PlanSequence{
							{
								Task: "those who resist our will",
							},
							{
								Do: &atc.PlanSequence{
									{
										Put:        "haters",
										Conditions: &atc.Conditions{"failure"},
									},
									{
										Put: "gonna",
									},
									{
										Put:        "hate",
										Conditions: &atc.Conditions{},
									},
								},
							},
						},
					}, resources, nil)).Should(Equal(atc.Plan{
						Compose: &atc.ComposePlan{
							A: atc.Plan{
								Task: &atc.TaskPlan{
									Name: "those who resist our will",
								},
							},
							B: atc.Plan{
								Conditional: &atc.ConditionalPlan{
									Conditions: atc.Conditions{"failure"},
									Plan: atc.Plan{
										Compose: &atc.ComposePlan{
											A: atc.Plan{
												Put: &atc.PutPlan{
													Name:     "haters",
													Resource: "haters",
												},
											},
											B: atc.Plan{
												Compose: &atc.ComposePlan{
													A: atc.Plan{
														Put: &atc.PutPlan{
															Name:     "gonna",
															Resource: "gonna",
														},
													},
													B: atc.Plan{
														Conditional: &atc.ConditionalPlan{
															Conditions: atc.Conditions{},
															Plan: atc.Plan{
																Put: &atc.PutPlan{
																	Name:     "hate",
																	Resource: "hate",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					}))
				})
			})
		})
	})

	Context("when the job has inputs, outputs, and a task config", func() {
		BeforeEach(func() {
			job.Privileged = true

			job.TaskConfigPath = "some-input/build.yml"
			job.TaskConfig = &atc.TaskConfig{
				Image: "some-image",
				Params: map[string]string{
					"FOO": "1",
					"BAR": "2",
				},
				Run: atc.TaskRunConfig{
					Path: "some-script",
					Args: []string{"arg1", "arg2"},
				},
			}

			job.InputConfigs = []atc.JobInputConfig{
				{
					RawName:  "some-input",
					Resource: "some-resource",
					Params:   atc.Params{"some": "params"},
				},
			}

			job.OutputConfigs = []atc.JobOutputConfig{
				{
					Resource:     "some-resource",
					Params:       atc.Params{"foo": "bar"},
					RawPerformOn: []atc.Condition{"success"},
				},
				{
					Resource:     "some-other-resource",
					Params:       atc.Params{"foo": "bar"},
					RawPerformOn: []atc.Condition{"failure"},
				},
				{
					Resource:     "some-other-other-resource",
					Params:       atc.Params{"foo": "bar"},
					RawPerformOn: []atc.Condition{},
				},
			}
		})

		It("creates a plan based on the job's configuration", func() {
			plan, err := factory.Create(job, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(plan).Should(Equal(expectedPlan))
		})

		Context("when no task config is present", func() {
			BeforeEach(func() {
				job.TaskConfig = nil
				job.TaskConfigPath = ""

				expectedPlan.Compose.B.Compose.B.Aggregate = &atc.AggregatePlan{
					"some-resource": atc.Plan{
						Put: &atc.PutPlan{
							Name:     "some-resource",
							Resource: "some-resource",
							Type:     "git",
							Params:   atc.Params{"foo": "bar"},
							Source:   atc.Source{"uri": "git://some-resource"},
						},
					},
					"some-other-resource": atc.Plan{
						Put: &atc.PutPlan{
							Name:     "some-other-resource",
							Resource: "some-other-resource",
							Type:     "git",
							Params:   atc.Params{"foo": "bar"},
							Source:   atc.Source{"uri": "git://some-other-resource"},
						},
					},
					"some-other-other-resource": atc.Plan{
						Put: &atc.PutPlan{
							Name:     "some-other-other-resource",
							Resource: "some-other-other-resource",
							Type:     "git",
							Params:   atc.Params{"foo": "bar"},
							Source:   atc.Source{"uri": "git://some-other-other-resource"},
						},
					},
				}
			})

			It("performs the outputs unconditionally", func() {
				plan, err := factory.Create(job, resources, nil)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(plan.Compose.B.Compose.B.Aggregate).Should(Equal(expectedPlan.Compose.B.Compose.B.Aggregate))
			})
		})

		Context("when an input has an explicit name", func() {
			BeforeEach(func() {
				job.InputConfigs = append(job.InputConfigs, atc.JobInputConfig{
					RawName:  "some-named-input",
					Resource: "some-named-resource",
					Params:   atc.Params{"some": "named-params"},
				})

				(*expectedPlan.Compose.A.Aggregate)["some-named-input"] = atc.Plan{
					Get: &atc.GetPlan{
						Name:     "some-named-input",
						Resource: "some-named-resource",
						Type:     "git",
						Source:   atc.Source{"uri": "git://some-named-resource"},
						Params:   atc.Params{"some": "named-params"},
					},
				}
			})

			It("uses it as the name for the input", func() {
				plan, err := factory.Create(job, resources, nil)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(plan.Compose.A.Aggregate).Should(Equal(expectedPlan.Compose.A.Aggregate))
			})
		})

		Context("when inputs with versions are specified", func() {
			It("uses them for the build's inputs", func() {
				plan, err := factory.Create(job, resources, []db.BuildInput{
					{
						Name: "some-input",
						VersionedResource: db.VersionedResource{
							Resource: "some-resource",
							Type:     "git-ng",
							Version:  db.Version{"version": "1"},
							Source:   db.Source{"uri": "git://some-provided-uri"},
						},
					},
				})
				Ω(err).ShouldNot(HaveOccurred())

				Ω((*plan.Compose.A.Aggregate)["some-input"].Get).Should(Equal(&atc.GetPlan{
					Name:     "some-input",
					Resource: "some-resource",
					Type:     "git-ng",
					Source:   atc.Source{"uri": "git://some-provided-uri"},
					Params:   atc.Params{"some": "params"},
					Version:  atc.Version{"version": "1"},
				}))
			})
		})

		Context("when the job's input is not found", func() {
			BeforeEach(func() {
				job.InputConfigs = append(job.InputConfigs, atc.JobInputConfig{
					Resource: "some-bogus-resource",
				})
			})

			It("returns an error", func() {
				_, err := factory.Create(job, resources, nil)
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("when the job's output is not found", func() {
			BeforeEach(func() {
				job.OutputConfigs = append(job.OutputConfigs, atc.JobOutputConfig{
					Resource: "some-bogus-resource",
				})
			})

			It("returns an error", func() {
				_, err := factory.Create(job, resources, nil)
				Ω(err).Should(HaveOccurred())
			})
		})
	})

	Context("when the job has both a plan and inputs/outputs", func() {
		BeforeEach(func() {
			job.Plan = atc.PlanSequence{{Get: "money"}, {Get: "paid"}}

			job.InputConfigs = []atc.JobInputConfig{
				{Resource: "money"},
			}
		})

		It("returns an error", func() {
			_, err := factory.Create(job, resources, nil)
			Ω(err).Should(HaveOccurred())
		})
	})
})
