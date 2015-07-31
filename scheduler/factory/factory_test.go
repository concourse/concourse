package factory_test

import (
	"github.com/concourse/atc"
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
		factory = &BuildFactory{
			PipelineName: "some-pipeline",
		}

		job = atc.JobConfig{
			Name: "some-job",
		}

		expectedPlan = atc.Plan{
			Compose: &atc.ComposePlan{
				A: atc.Plan{
					Aggregate: &atc.AggregatePlan{
						atc.Plan{
							Get: &atc.GetPlan{
								Type:     "git",
								Name:     "some-input",
								Resource: "some-resource",
								Pipeline: "some-pipeline",
								Tags:     []string{"some", "tags"},
								Source:   atc.Source{"uri": "git://some-resource"},
								Params:   atc.Params{"some": "params"},
							},
						},
					},
				},
				B: atc.Plan{
					Conditional: &atc.ConditionalPlan{
						Conditions: []atc.Condition{atc.ConditionSuccess},
						Plan: atc.Plan{
							Compose: &atc.ComposePlan{
								A: atc.Plan{
									Task: &atc.TaskPlan{
										Name: "build",

										Privileged: true,
										Tags:       []string{"some", "tags"},

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
										atc.Plan{
											Conditional: &atc.ConditionalPlan{
												Conditions: []atc.Condition{atc.ConditionSuccess},
												Plan: atc.Plan{
													OnSuccess: &atc.OnSuccessPlan{
														Step: atc.Plan{
															Put: &atc.PutPlan{
																Name:     "some-resource",
																Resource: "some-resource",
																Pipeline: "some-pipeline",
																Type:     "git",
																Tags:     []string{"some", "tags"},
																Params:   atc.Params{"foo": "bar"},
																Source:   atc.Source{"uri": "git://some-resource"},
															},
														},
														Next: atc.Plan{
															DependentGet: &atc.DependentGetPlan{
																Name:     "some-resource",
																Resource: "some-resource",
																Pipeline: "some-pipeline",
																Type:     "git",
																Tags:     []string{"some", "tags"},
																Params:   atc.Params{"fooGet": "barGet"},
															},
														},
													},
												},
											},
										},
										atc.Plan{
											Conditional: &atc.ConditionalPlan{
												Conditions: []atc.Condition{atc.ConditionFailure},
												Plan: atc.Plan{
													OnSuccess: &atc.OnSuccessPlan{
														Step: atc.Plan{
															Put: &atc.PutPlan{
																Name:     "some-other-resource",
																Resource: "some-other-resource",
																Pipeline: "some-pipeline",
																Type:     "git",
																Params:   atc.Params{"foo": "bar"},
																Source:   atc.Source{"uri": "git://some-other-resource"},
															},
														},
														Next: atc.Plan{
															DependentGet: &atc.DependentGetPlan{
																Name:     "some-other-resource",
																Resource: "some-other-resource",
																Pipeline: "some-pipeline",
																Type:     "git",
																Params:   atc.Params{"fooGet": "barGet"},
															},
														},
													},
												},
											},
										},
										atc.Plan{
											Conditional: &atc.ConditionalPlan{
												Conditions: []atc.Condition{},
												Plan: atc.Plan{
													OnSuccess: &atc.OnSuccessPlan{
														Step: atc.Plan{
															Put: &atc.PutPlan{
																Name:     "some-other-other-resource",
																Resource: "some-other-other-resource",
																Pipeline: "some-pipeline",
																Type:     "git",
																Params:   atc.Params{"foo": "bar"},
																Source:   atc.Source{"uri": "git://some-other-other-resource"},
															},
														},
														Next: atc.Plan{
															DependentGet: &atc.DependentGetPlan{
																Name:     "some-other-other-resource",
																Resource: "some-other-other-resource",
																Pipeline: "some-pipeline",
																Type:     "git",
																Params:   atc.Params{"fooGet": "barGet"},
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

	Context("when the job has no plan", func() {
		It("returns an empty plan", func() {
			Ω(factory.Create(job, resources, nil)).Should(Equal(atc.Plan{}))
		})
	})

	// Context("when the job has a plan", func() { // to take over the world
	// BeforeEach(func() {
	// 	job.Plan = atc.PlanSequence{
	// 		{
	// 			Aggregate: &atc.PlanSequence{
	// 				{
	// 					Get:      "some-input",
	// 					Resource: "some-resource",
	// 					Params:   atc.Params{"some": "params"},
	// 					Tags:     []string{"some", "tags"},
	// 				},
	// 			},
	// 		},
	// 		{
	// 			Task:           "build",
	// 			Privileged:     true,
	// 			Tags:           []string{"some", "tags"},
	// 			TaskConfigPath: "some-input/build.yml",
	// 			TaskConfig: &atc.TaskConfig{
	// 				Image: "some-image",
	// 				Params: map[string]string{
	// 					"FOO": "1",
	// 					"BAR": "2",
	// 				},
	// 				Run: atc.TaskRunConfig{
	// 					Path: "some-script",
	// 					Args: []string{"arg1", "arg2"},
	// 				},
	// 			},
	// 		},
	// 		{
	// 			Aggregate: &atc.PlanSequence{
	// 				{
	// 					Conditions: &atc.Conditions{atc.ConditionSuccess},
	// 					RawName:    "some-resource",
	// 					Do: &atc.PlanSequence{
	// 						{
	// 							Put:       "some-resource",
	// 							Params:    atc.Params{"foo": "bar"},
	// 							Tags:      []string{"some", "tags"},
	// 							GetParams: atc.Params{"fooGet": "barGet"},
	// 						},
	// 					},
	// 				},
	// 				{
	// 					Conditions: &atc.Conditions{atc.ConditionFailure},
	// 					Put:        "some-other-resource",
	// 					Params:     atc.Params{"foo": "bar"},
	// 					GetParams:  atc.Params{"fooGet": "barGet"},
	// 				},
	// 				{
	// 					Conditions: &atc.Conditions{},
	// 					Put:        "some-other-other-resource",
	// 					Params:     atc.Params{"foo": "bar"},
	// 					GetParams:  atc.Params{"fooGet": "barGet"},
	// 				},
	// 			},
	// 		},
	// 	}
	// })

	// It("uses the plan in the job config if present", func() {
	// 	plan, err := factory.Create(job, resources, nil)

	// 	Ω(err).ShouldNot(HaveOccurred())
	// 	Ω(plan).Should(Equal(expectedPlan))
	// })

	// Describe("chains of conditional plans", func() {
	// 	It("breaks the chain at each condition", func() {
	// 		Ω(factory.Create(atc.JobConfig{
	// 			Plan: atc.PlanSequence{
	// 				{
	// 					Task: "those who resist our will",
	// 				},
	// 				{
	// 					Task:       "the deal",
	// 					Conditions: &atc.Conditions{atc.ConditionSuccess},
	// 				},
	// 				{
	// 					Task:       "the other deal",
	// 					Conditions: &atc.Conditions{atc.ConditionFailure},
	// 				},
	// 			},
	// 		}, resources, nil)).Should(Equal(atc.Plan{
	// 			Compose: &atc.ComposePlan{
	// 				A: atc.Plan{
	// 					Task: &atc.TaskPlan{
	// 						Name: "those who resist our will",
	// 					},
	// 				},
	// 				B: atc.Plan{
	// 					Conditional: &atc.ConditionalPlan{
	// 						Conditions: atc.Conditions{atc.ConditionSuccess},
	// 						Plan: atc.Plan{
	// 							Compose: &atc.ComposePlan{
	// 								A: atc.Plan{
	// 									Task: &atc.TaskPlan{
	// 										Name: "the deal",
	// 									},
	// 								},
	// 								B: atc.Plan{
	// 									Conditional: &atc.ConditionalPlan{
	// 										Conditions: atc.Conditions{atc.ConditionFailure},
	// 										Plan: atc.Plan{
	// 											Task: &atc.TaskPlan{
	// 												Name: "the other deal",
	// 											},
	// 										},
	// 									},
	// 								},
	// 							},
	// 						},
	// 					},
	// 				},
	// 			},
	// 		}))
	// 	})
	// })

	// Context("when it has an explicit condition", func() {
	// 	It("runs with the given condition", func() {
	// 		Ω(factory.Create(atc.JobConfig{
	// 			Plan: atc.PlanSequence{
	// 				{
	// 					Task: "those who resist our will",
	// 				},
	// 				{
	// 					Get:        "money",
	// 					Conditions: &atc.Conditions{atc.ConditionFailure},
	// 				},
	// 			},
	// 		}, resources, nil)).Should(Equal(atc.Plan{
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
	// 							Get: &atc.GetPlan{
	// 								Name:     "money",
	// 								Resource: "money",
	// 								Pipeline: "some-pipeline",
	// 							},
	// 						},
	// 					},
	// 				},
	// 			},
	// 		}))

	// })
	// })
	// })

	Context("when a task plan follows a task plan", func() {
		Context("when it has an explicit condition", func() {
			It("runs with the given condition", func() {
				input := atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Task: "those who resist our will",
						},
						{
							Task:       "haters",
							Conditions: &atc.Conditions{atc.ConditionFailure},
						},
					},
				}

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
										Name: "haters",
									},
								},
							},
						},
					},
				}

				actual, err := factory.Create(input, resources, nil)
				Ω(err).NotTo(HaveOccurred())

				Ω(actual).Should(Equal(expected))
			})
		})
	})

	// Context("when an aggregate plan follows a task plan", func() {
	// 	Context("and any of its plans have explicit conditions", func() {
	// 		It("runs them with their given condition", func() {
	// 			input := atc.JobConfig{
	// 				Plan: atc.PlanSequence{
	// 					{
	// 						Task: "those who resist our will",
	// 					},
	// 					{
	// 						Aggregate: &atc.PlanSequence{
	// 							{
	// 								Put:        "haters",
	// 								Conditions: &atc.Conditions{atc.ConditionFailure},
	// 							},
	// 							{
	// 								Put: "gonna",
	// 							},
	// 							{
	// 								Put:        "hate",
	// 								Conditions: &atc.Conditions{},
	// 							},
	// 						},
	// 					},
	// 				},
	// 			}
	// 			expected := atc.Plan{
	// 				Compose: &atc.ComposePlan{
	// 					A: atc.Plan{
	// 						Task: &atc.TaskPlan{
	// 							Name: "those who resist our will",
	// 						},
	// 					},
	// 					B: atc.Plan{
	// 						Aggregate: &atc.AggregatePlan{
	// 							atc.Plan{
	// 								Conditional: &atc.ConditionalPlan{
	// 									Conditions: atc.Conditions{atc.ConditionFailure},
	// 									Plan: atc.Plan{
	// 										OnSuccess: &atc.OnSuccessPlan{
	// 											Step: atc.Plan{
	// 												Put: &atc.PutPlan{
	// 													Name:     "haters",
	// 													Resource: "haters",
	// 													Pipeline: "some-pipeline",
	// 												},
	// 											},
	// 											Next: atc.Plan{
	// 												DependentGet: &atc.DependentGetPlan{
	// 													Name:     "haters",
	// 													Resource: "haters",
	// 													Pipeline: "some-pipeline",
	// 												},
	// 											},
	// 										},
	// 									},
	// 								},
	// 							},
	// 							atc.Plan{
	// 								Conditional: &atc.ConditionalPlan{
	// 									Conditions: atc.Conditions{atc.ConditionSuccess},
	// 									Plan: atc.Plan{
	// 										OnSuccess: &atc.OnSuccessPlan{
	// 											Step: atc.Plan{
	// 												Put: &atc.PutPlan{
	// 													Name:     "gonna",
	// 													Resource: "gonna",
	// 													Pipeline: "some-pipeline",
	// 												},
	// 											},
	// 											Next: atc.Plan{
	// 												DependentGet: &atc.DependentGetPlan{
	// 													Name:     "gonna",
	// 													Resource: "gonna",
	// 													Pipeline: "some-pipeline",
	// 												},
	// 											},
	// 										},
	// 									},
	// 								},
	// 							},
	// 							atc.Plan{
	// 								Conditional: &atc.ConditionalPlan{
	// 									Conditions: atc.Conditions{},
	// 									Plan: atc.Plan{
	// 										OnSuccess: &atc.OnSuccessPlan{
	// 											Step: atc.Plan{
	// 												Put: &atc.PutPlan{
	// 													Name:     "hate",
	// 													Resource: "hate",
	// 													Pipeline: "some-pipeline",
	// 												},
	// 											},
	// 											Next: atc.Plan{
	// 												DependentGet: &atc.DependentGetPlan{
	// 													Name:     "hate",
	// 													Resource: "hate",
	// 													Pipeline: "some-pipeline",
	// 												},
	// 											},
	// 										},
	// 									},
	// 								},
	// 							},
	// 						},
	// 					},
	// 				},
	// 			}

	// 			actual, err := factory.Create(input, resources, nil)
	// 			Ω(err).NotTo(HaveOccurred())

	// 			Ω(actual).Should(Equal(expected))
	// 		})
	// 	})
	// })

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
									Get: "haters",
								},
								{
									Get: "gonna",
								},
								{
									Get:        "hate",
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
								Conditions: atc.Conditions{atc.ConditionSuccess},
								Plan: atc.Plan{
									Compose: &atc.ComposePlan{
										A: atc.Plan{
											Get: &atc.GetPlan{
												Name:     "haters",
												Resource: "haters",
												Pipeline: "some-pipeline",
											},
										},
										B: atc.Plan{
											Conditional: &atc.ConditionalPlan{
												Conditions: atc.Conditions{atc.ConditionSuccess},
												Plan: atc.Plan{
													Compose: &atc.ComposePlan{
														A: atc.Plan{
															Get: &atc.GetPlan{
																Name:     "gonna",
																Resource: "gonna",
																Pipeline: "some-pipeline",
															},
														},
														B: atc.Plan{
															Conditional: &atc.ConditionalPlan{
																Conditions: atc.Conditions{},
																Plan: atc.Plan{
																	Get: &atc.GetPlan{
																		Name:     "hate",
																		Resource: "hate",
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
									Get:        "haters",
									Conditions: &atc.Conditions{atc.ConditionFailure},
								},
								{
									Get: "gonna",
								},
								{
									Get:        "hate",
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
								Conditions: atc.Conditions{atc.ConditionFailure},
								Plan: atc.Plan{
									Compose: &atc.ComposePlan{
										A: atc.Plan{
											Get: &atc.GetPlan{
												Name:     "haters",
												Resource: "haters",
												Pipeline: "some-pipeline",
											},
										},
										B: atc.Plan{
											Conditional: &atc.ConditionalPlan{
												Conditions: atc.Conditions{atc.ConditionSuccess},
												Plan: atc.Plan{
													Compose: &atc.ComposePlan{
														A: atc.Plan{
															Get: &atc.GetPlan{
																Name:     "gonna",
																Resource: "gonna",
																Pipeline: "some-pipeline",
															},
														},
														B: atc.Plan{
															Conditional: &atc.ConditionalPlan{
																Conditions: atc.Conditions{},
																Plan: atc.Plan{
																	Get: &atc.GetPlan{
																		Name:     "hate",
																		Resource: "hate",
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
								},
							},
						},
					},
				}))
			})
		})
	})
})
