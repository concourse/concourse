package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Hooks", func() {
	var (
		buildFactory factory.BuildFactory

		resources           atc.ResourceConfigs
		actualPlanFactory   atc.PlanFactory
		expectedPlanFactory atc.PlanFactory
	)

	BeforeEach(func() {
		actualPlanFactory = atc.NewPlanFactory(123)
		expectedPlanFactory = atc.NewPlanFactory(123)
		buildFactory = factory.NewBuildFactory("some-pipeline", actualPlanFactory)

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
			actual := buildFactory.Create(input, resources, nil)

			expected := expectedPlanFactory.NewPlan(atc.OnFailurePlan{
				Step: expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
					Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "those who resist our will",
						Pipeline: "some-pipeline",
					}),
					Next: expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
						Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:     "those who also resist our will",
							Pipeline: "some-pipeline",
						}),
						Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:     "third task",
							Pipeline: "some-pipeline",
						}),
					}),
				}),
				Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "some other failure",
					Pipeline: "some-pipeline",
				}),
			})

			Expect(actual).To(Equal(expected))
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
			actual := buildFactory.Create(input, resources, nil)

			expected := expectedPlanFactory.NewPlan(atc.OnFailurePlan{
				Step: expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
					Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "those who resist our will",
						Pipeline: "some-pipeline",
					}),
					Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "those who also resist our will",
						Pipeline: "some-pipeline",
					}),
				}),
				Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "some other failure",
					Pipeline: "some-pipeline",
				}),
			})

			Expect(actual).To(Equal(expected))
		})
	})

	Context("when I have an empty plan", func() {
		It("returns an empty plan", func() {
			actual := buildFactory.Create(atc.JobConfig{}, resources, nil)

			expected := atc.Plan{}
			Expect(actual).To(Equal(expected))
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
			actual := buildFactory.Create(input, resources, nil)

			expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
				Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "some-task",
					Pipeline: "some-pipeline",
				}),
				Next: expectedPlanFactory.NewPlan(atc.AggregatePlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "agg-task-1",
						Pipeline: "some-pipeline",
					}),
					expectedPlanFactory.NewPlan(atc.AggregatePlan{
						expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:     "agg-agg-task-1",
							Pipeline: "some-pipeline",
						}),
					}),
				}),
			})

			Expect(actual).To(Equal(expected))
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
			actual := buildFactory.Create(input, resources, nil)

			expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
				Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "some-task",
					Pipeline: "some-pipeline",
				}),
				Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "do-task-1",
					Pipeline: "some-pipeline",
				}),
			})

			Expect(actual).To(Equal(expected))
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
			actual := buildFactory.Create(input, resources, nil)

			expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
				Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "some-task",
					Pipeline: "some-pipeline",
				}),
				Next: expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
					Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "do-task-1",
						Pipeline: "some-pipeline",
					}),
					Next: expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
						Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:     "do-task-2",
							Pipeline: "some-pipeline",
						}),
						Next: expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
							Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
								Name:     "do-task-3",
								Pipeline: "some-pipeline",
							}),
							Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
								Name:     "do-task-4",
								Pipeline: "some-pipeline",
							}),
						}),
					}),
				}),
			})

			Expect(actual).To(Equal(expected))
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
			actual := buildFactory.Create(input, resources, nil)

			expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
				Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "some-task",
					Pipeline: "some-pipeline",
				}),
				Next: expectedPlanFactory.NewPlan(atc.AggregatePlan{
					expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
						Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:     "agg-task-1",
							Pipeline: "some-pipeline",
						}),
						Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:     "agg-task-1-success",
							Pipeline: "some-pipeline",
						}),
					}),
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "agg-task-2",
						Pipeline: "some-pipeline",
					}),
				}),
			})

			Expect(actual).To(Equal(expected))
		})
	})

	Context("when I have hooks in my plan", func() {
		It("can build a job with one failure hook", func() {
			actual := buildFactory.Create(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "those who resist our will",
						Failure: &atc.PlanConfig{
							Get: "some-resource",
						},
					},
				},
			}, resources, nil)

			expected := expectedPlanFactory.NewPlan(atc.OnFailurePlan{
				Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "those who resist our will",
					Pipeline: "some-pipeline",
				}),
				Next: expectedPlanFactory.NewPlan(atc.GetPlan{
					Name:     "some-resource",
					Type:     "git",
					Resource: "some-resource",
					Pipeline: "some-pipeline",
					Source: atc.Source{
						"uri": "git://some-resource",
					},
				}),
			})

			Expect(actual).To(Equal(expected))
		})

		It("can build a job with one failure hook that has a timeout", func() {
			actual := buildFactory.Create(atc.JobConfig{
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

			expected := expectedPlanFactory.NewPlan(atc.OnFailurePlan{
				Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "those who resist our will",
					Pipeline: "some-pipeline",
				}),
				Next: expectedPlanFactory.NewPlan(atc.TimeoutPlan{
					Duration: "10s",
					Step: expectedPlanFactory.NewPlan(atc.GetPlan{
						Name:     "some-resource",
						Type:     "git",
						Resource: "some-resource",
						Pipeline: "some-pipeline",
						Source: atc.Source{
							"uri": "git://some-resource",
						},
					}),
				}),
			})

			Expect(actual).To(Equal(expected))
		})

		It("can build a job with multiple failure hooks", func() {
			actual := buildFactory.Create(atc.JobConfig{
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

			expected := expectedPlanFactory.NewPlan(atc.OnFailurePlan{
				Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "those who resist our will",
					Pipeline: "some-pipeline",
				}),
				Next: expectedPlanFactory.NewPlan(atc.OnFailurePlan{
					Step: expectedPlanFactory.NewPlan(atc.GetPlan{
						Name:     "some-resource",
						Type:     "git",
						Resource: "some-resource",
						Pipeline: "some-pipeline",
						Source: atc.Source{
							"uri": "git://some-resource",
						},
					}),
					Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "those who still resist our will",
						Pipeline: "some-pipeline",
					}),
				}),
			})

			Expect(actual).To(Equal(expected))
		})

		It("can build a job with multiple ensure and failure hooks", func() {
			actual := buildFactory.Create(atc.JobConfig{
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

			expected := expectedPlanFactory.NewPlan(atc.OnFailurePlan{
				Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "those who resist our will",
					Pipeline: "some-pipeline",
				}),
				Next: expectedPlanFactory.NewPlan(atc.EnsurePlan{
					Step: expectedPlanFactory.NewPlan(atc.GetPlan{
						Name:     "some-resource",
						Type:     "git",
						Resource: "some-resource",
						Pipeline: "some-pipeline",
						Source: atc.Source{
							"uri": "git://some-resource",
						},
					}),
					Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "those who still resist our will",
						Pipeline: "some-pipeline",
					}),
				}),
			})

			Expect(actual).To(Equal(expected))
		})

		It("can build a job with failure, success and ensure hooks at the same level", func() {
			actual := buildFactory.Create(atc.JobConfig{
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

			expected := expectedPlanFactory.NewPlan(atc.EnsurePlan{
				Step: expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
					Step: expectedPlanFactory.NewPlan(atc.OnFailurePlan{
						Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:     "those who resist our will",
							Pipeline: "some-pipeline",
						}),
						Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:     "those who failed to resist our will",
							Pipeline: "some-pipeline",
						}),
					}),
					Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "those who successfully resisted our will",
						Pipeline: "some-pipeline",
					}),
				}),
				Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:     "those who always resist our will",
					Pipeline: "some-pipeline",
				}),
			})

			Expect(actual).To(Equal(expected))
		})

		It("can build a job with multiple ensure, failure and success hooks", func() {
			actual := buildFactory.Create(atc.JobConfig{
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

			expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
				Step: expectedPlanFactory.NewPlan(atc.OnFailurePlan{
					Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "those who resist our will",
						Pipeline: "some-pipeline",
					}),
					Next: expectedPlanFactory.NewPlan(atc.EnsurePlan{
						Step: expectedPlanFactory.NewPlan(atc.GetPlan{
							Name:     "some-resource",
							Type:     "git",
							Resource: "some-resource",
							Pipeline: "some-pipeline",
							Source: atc.Source{
								"uri": "git://some-resource",
							},
						}),
						Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:     "those who still resist our will",
							Pipeline: "some-pipeline",
						}),
					}),
				}),
				Next: expectedPlanFactory.NewPlan(atc.GetPlan{
					Name:     "some-resource",
					Type:     "git",
					Resource: "some-resource",
					Pipeline: "some-pipeline",
					Source: atc.Source{
						"uri": "git://some-resource",
					},
				}),
			})

			Expect(actual).To(Equal(expected))
		})

		Context("and multiple steps in my plan", func() {
			It("can build a job with a task with hooks then 2 more tasks", func() {
				actual := buildFactory.Create(atc.JobConfig{
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

				expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
					Step: expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
						Step: expectedPlanFactory.NewPlan(atc.OnFailurePlan{
							Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
								Name:     "those who resist our will",
								Pipeline: "some-pipeline",
							}),
							Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
								Name:     "some other task",
								Pipeline: "some-pipeline",
							}),
						}),
						Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:     "some other success task",
							Pipeline: "some-pipeline",
						}),
					}),
					Next: expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
						Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:     "those who still resist our will",
							Pipeline: "some-pipeline",
						}),
						Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:     "shall be defeated",
							Pipeline: "some-pipeline",
						}),
					}),
				})
				Expect(actual).To(Equal(expected))
			})

			It("can build a job with a task then a do", func() {
				actual := buildFactory.Create(atc.JobConfig{
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

				expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
					Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "those who start resisting our will",
						Pipeline: "some-pipeline",
					}),
					Next: expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
						Step: expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
							Step: expectedPlanFactory.NewPlan(atc.OnFailurePlan{
								Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
									Name:     "those who resist our will",
									Pipeline: "some-pipeline",
								}),
								Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
									Name:     "some other task",
									Pipeline: "some-pipeline",
								}),
							}),
							Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
								Name:     "some other success task",
								Pipeline: "some-pipeline",
							}),
						}),
						Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:     "those who used to resist our will",
							Pipeline: "some-pipeline",
						}),
					}),
				})
				Expect(actual).To(Equal(expected))
			})

			It("can build a job with a do then a task", func() {
				actual := buildFactory.Create(atc.JobConfig{
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

				expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
					Step: expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
						Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:     "those who resist our will",
							Pipeline: "some-pipeline",
						}),
						Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:     "those who used to resist our will",
							Pipeline: "some-pipeline",
						}),
					}),

					Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "those who start resisting our will",
						Pipeline: "some-pipeline",
					}),
				})

				Expect(actual).To(Equal(expected))
			})
		})
	})
})
