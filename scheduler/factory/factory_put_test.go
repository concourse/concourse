package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"
	"github.com/concourse/atc/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Put", func() {
	Describe("Put/DependentGet locations", func() {
		var (
			buildFactory factory.BuildFactory

			resources           atc.ResourceConfigs
			input               atc.JobConfig
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

		Context("with a put at the top-level", func() {
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
				Expect(err).NotTo(HaveOccurred())

				expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
					Step: expectedPlanFactory.NewPlan(atc.PutPlan{
						Type:     "git",
						Name:     "some-put",
						Resource: "some-resource",
						Pipeline: "some-pipeline",
						Source: atc.Source{
							"uri": "git://some-resource",
						},
					}),
					Next: expectedPlanFactory.NewPlan(atc.DependentGetPlan{
						Type:     "git",
						Name:     "some-put",
						Resource: "some-resource",
						Pipeline: "some-pipeline",
						Source: atc.Source{
							"uri": "git://some-resource",
						},
					}),
				})
				Expect(actual).To(testhelpers.MatchPlan(expected))
			})
		})

		Context("with a put for a non-existent resource", func() {
			BeforeEach(func() {
				input = atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Put:      "some-put",
							Resource: "what-resource",
						},
					},
				}
			})

			It("returns the correct error", func() {
				_, err := buildFactory.Create(input, resources, nil)
				Expect(err).To(Equal(factory.ErrResourceNotFound))
			})
		})
	})

	Describe("Put/DependentGet build plan", func() {
		var (
			buildFactory factory.BuildFactory

			resources           atc.ResourceConfigs
			input               atc.JobConfig
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
				Expect(err).NotTo(HaveOccurred())

				expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
					Step: expectedPlanFactory.NewPlan(atc.PutPlan{
						Type:     "git",
						Name:     "some-put",
						Resource: "some-resource",
						Pipeline: "some-pipeline",
						Source: atc.Source{
							"uri": "git://some-resource",
						},
					}),
					Next: expectedPlanFactory.NewPlan(atc.DependentGetPlan{
						Type:     "git",
						Name:     "some-put",
						Resource: "some-resource",
						Pipeline: "some-pipeline",
						Source: atc.Source{
							"uri": "git://some-resource",
						},
					}),
				})
				Expect(actual).To(testhelpers.MatchPlan(expected))
			})
		})

		Context("when I have a put in a hook", func() {
			BeforeEach(func() {
				input = atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Task: "some-task",
							Success: &atc.PlanConfig{
								Put: "some-resource",
							},
						},
					},
				}
			})

			It("returns the correct plan", func() {
				actual, err := buildFactory.Create(input, resources, nil)
				Expect(err).NotTo(HaveOccurred())

				expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
					Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "some-task",
						Pipeline: "some-pipeline",
					}),

					Next: expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
						Step: expectedPlanFactory.NewPlan(atc.PutPlan{
							Type:     "git",
							Name:     "some-resource",
							Resource: "some-resource",
							Pipeline: "some-pipeline",
							Source: atc.Source{
								"uri": "git://some-resource",
							},
						}),
						Next: expectedPlanFactory.NewPlan(atc.DependentGetPlan{
							Type:     "git",
							Name:     "some-resource",
							Resource: "some-resource",
							Pipeline: "some-pipeline",
							Source: atc.Source{
								"uri": "git://some-resource",
							},
						}),
					}),
				})
				Expect(actual).To(testhelpers.MatchPlan(expected))
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
									Put: "some-resource",
								},
							},
						},
					},
				}
			})

			It("returns the correct plan", func() {
				actual, err := buildFactory.Create(input, resources, nil)
				Expect(err).NotTo(HaveOccurred())

				expected := expectedPlanFactory.NewPlan(atc.AggregatePlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "some thing",
						Pipeline: "some-pipeline",
					}),
					expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
						Step: expectedPlanFactory.NewPlan(atc.PutPlan{
							Type:     "git",
							Name:     "some-resource",
							Resource: "some-resource",
							Pipeline: "some-pipeline",
							Source: atc.Source{
								"uri": "git://some-resource",
							},
						}),
						Next: expectedPlanFactory.NewPlan(atc.DependentGetPlan{
							Type:     "git",
							Name:     "some-resource",
							Resource: "some-resource",
							Pipeline: "some-pipeline",
							Source: atc.Source{
								"uri": "git://some-resource",
							},
						}),
					}),
				})
				Expect(actual).To(testhelpers.MatchPlan(expected))
			})
		})

		Context("when a put plan follows a task plan", func() {
			BeforeEach(func() {
				input = atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Task: "some-task",
						},
						{
							Put:      "money",
							Resource: "some-resource",
						},
					},
				}
			})

			It("returns the correct plan", func() {
				actual, err := buildFactory.Create(input, resources, nil)
				Expect(err).NotTo(HaveOccurred())

				expected := expectedPlanFactory.NewPlan(atc.DoPlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "some-task",
						Pipeline: "some-pipeline",
					}),
					expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
						Step: expectedPlanFactory.NewPlan(atc.PutPlan{
							Type:     "git",
							Name:     "money",
							Resource: "some-resource",
							Pipeline: "some-pipeline",
							Source: atc.Source{
								"uri": "git://some-resource",
							},
						}),
						Next: expectedPlanFactory.NewPlan(atc.DependentGetPlan{
							Type:     "git",
							Name:     "money",
							Resource: "some-resource",
							Pipeline: "some-pipeline",
							Source: atc.Source{
								"uri": "git://some-resource",
							},
						}),
					}),
				})

				Expect(actual).To(testhelpers.MatchPlan(expected))
			})
		})

		Context("when a put plan is between two task plans", func() {
			BeforeEach(func() {
				input = atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Task: "those who resist our will",
						},
						{
							Put: "some-resource",
						},
						{
							Task: "some-other-task",
						},
					},
				}
			})

			It("returns the correct plan", func() {
				expectedPlan := expectedPlanFactory.NewPlan(atc.DoPlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "those who resist our will",
						Pipeline: "some-pipeline",
					}),
					expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
						Step: expectedPlanFactory.NewPlan(atc.PutPlan{
							Type:     "git",
							Name:     "some-resource",
							Resource: "some-resource",
							Pipeline: "some-pipeline",
							Source: atc.Source{
								"uri": "git://some-resource",
							},
							Params: nil,
						}),
						Next: expectedPlanFactory.NewPlan(atc.DependentGetPlan{
							Type:     "git",
							Name:     "some-resource",
							Resource: "some-resource",
							Pipeline: "some-pipeline",
							Source: atc.Source{
								"uri": "git://some-resource",
							},
						}),
					}),
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:     "some-other-task",
						Pipeline: "some-pipeline",
					}),
				})

				actual, err := buildFactory.Create(input, resources, nil)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(testhelpers.MatchPlan(expectedPlan))
			})
		})
	})
})
