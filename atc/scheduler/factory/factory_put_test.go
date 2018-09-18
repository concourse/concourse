package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"
	"github.com/concourse/atc/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Put", func() {
	Describe("Put/Get locations", func() {
		var (
			buildFactory factory.BuildFactory

			resources           atc.ResourceConfigs
			resourceTypes       atc.VersionedResourceTypes
			input               atc.JobConfig
			actualPlanFactory   atc.PlanFactory
			expectedPlanFactory atc.PlanFactory
		)

		BeforeEach(func() {
			actualPlanFactory = atc.NewPlanFactory(123)
			expectedPlanFactory = atc.NewPlanFactory(123)
			buildFactory = factory.NewBuildFactory(42, actualPlanFactory)

			resources = atc.ResourceConfigs{
				{
					Name:   "some-resource",
					Type:   "git",
					Source: atc.Source{"uri": "git://some-resource"},
				},
			}

			resourceTypes = atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Name:   "some-custom-resource",
						Type:   "registry-image",
						Source: atc.Source{"some": "custom-source"},
					},
					Version: atc.Version{"some": "version"},
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
				actual, err := buildFactory.Create(input, resources, resourceTypes, nil)
				Expect(err).NotTo(HaveOccurred())

				putPlan := expectedPlanFactory.NewPlan(atc.PutPlan{
					Type:     "git",
					Name:     "some-put",
					Resource: "some-resource",
					Source: atc.Source{
						"uri": "git://some-resource",
					},
					VersionedResourceTypes: resourceTypes,
				})

				expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
					Step: putPlan,
					Next: expectedPlanFactory.NewPlan(atc.GetPlan{
						Type:     "git",
						Name:     "some-put",
						Resource: "some-resource",
						Source: atc.Source{
							"uri": "git://some-resource",
						},
						VersionFrom:            &putPlan.ID,
						VersionedResourceTypes: resourceTypes,
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
				_, err := buildFactory.Create(input, resources, resourceTypes, nil)
				Expect(err).To(Equal(factory.ErrResourceNotFound))
			})
		})
	})

	Describe("Put/Get build plan", func() {
		var (
			buildFactory factory.BuildFactory

			resources           atc.ResourceConfigs
			resourceTypes       atc.VersionedResourceTypes
			input               atc.JobConfig
			actualPlanFactory   atc.PlanFactory
			expectedPlanFactory atc.PlanFactory
		)

		BeforeEach(func() {
			actualPlanFactory = atc.NewPlanFactory(123)
			expectedPlanFactory = atc.NewPlanFactory(123)
			buildFactory = factory.NewBuildFactory(42, actualPlanFactory)

			resources = atc.ResourceConfigs{
				{
					Name:   "some-resource",
					Type:   "git",
					Source: atc.Source{"uri": "git://some-resource"},
				},
			}

			resourceTypes = atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Name:   "some-resource-type",
						Type:   "some-underlying-type",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
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
				actual, err := buildFactory.Create(input, resources, resourceTypes, nil)
				Expect(err).NotTo(HaveOccurred())

				putPlan := expectedPlanFactory.NewPlan(atc.PutPlan{
					Type:     "git",
					Name:     "some-put",
					Resource: "some-resource",
					Source: atc.Source{
						"uri": "git://some-resource",
					},
					VersionedResourceTypes: resourceTypes,
				})
				expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
					Step: putPlan,
					Next: expectedPlanFactory.NewPlan(atc.GetPlan{
						Type:     "git",
						Name:     "some-put",
						Resource: "some-resource",
						Source: atc.Source{
							"uri": "git://some-resource",
						},
						VersionFrom:            &putPlan.ID,
						VersionedResourceTypes: resourceTypes,
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
				actual, err := buildFactory.Create(input, resources, resourceTypes, nil)
				Expect(err).NotTo(HaveOccurred())

				putPlan := expectedPlanFactory.NewPlan(atc.PutPlan{
					Type:     "git",
					Name:     "some-resource",
					Resource: "some-resource",
					Source: atc.Source{
						"uri": "git://some-resource",
					},
					VersionedResourceTypes: resourceTypes,
				})

				expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
					Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "some-task",
						VersionedResourceTypes: resourceTypes,
					}),

					Next: expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
						Step: putPlan,
						Next: expectedPlanFactory.NewPlan(atc.GetPlan{
							Type:     "git",
							Name:     "some-resource",
							Resource: "some-resource",
							Source: atc.Source{
								"uri": "git://some-resource",
							},
							VersionFrom:            &putPlan.ID,
							VersionedResourceTypes: resourceTypes,
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
				actual, err := buildFactory.Create(input, resources, resourceTypes, nil)
				Expect(err).NotTo(HaveOccurred())

				putPlan := expectedPlanFactory.NewPlan(atc.PutPlan{
					Type:     "git",
					Name:     "some-resource",
					Resource: "some-resource",
					Source: atc.Source{
						"uri": "git://some-resource",
					},
					VersionedResourceTypes: resourceTypes,
				})

				expected := expectedPlanFactory.NewPlan(atc.AggregatePlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "some thing",
						VersionedResourceTypes: resourceTypes,
					}),
					expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
						Step: putPlan,
						Next: expectedPlanFactory.NewPlan(atc.GetPlan{
							Type:     "git",
							Name:     "some-resource",
							Resource: "some-resource",
							Source: atc.Source{
								"uri": "git://some-resource",
							},
							VersionFrom:            &putPlan.ID,
							VersionedResourceTypes: resourceTypes,
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
				actual, err := buildFactory.Create(input, resources, resourceTypes, nil)
				Expect(err).NotTo(HaveOccurred())

				putPlan := expectedPlanFactory.NewPlan(atc.PutPlan{
					Type:     "git",
					Name:     "money",
					Resource: "some-resource",
					Source: atc.Source{
						"uri": "git://some-resource",
					},
					VersionedResourceTypes: resourceTypes,
				})

				expected := expectedPlanFactory.NewPlan(atc.DoPlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "some-task",
						VersionedResourceTypes: resourceTypes,
					}),
					expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
						Step: putPlan,
						Next: expectedPlanFactory.NewPlan(atc.GetPlan{
							Type:     "git",
							Name:     "money",
							Resource: "some-resource",
							Source: atc.Source{
								"uri": "git://some-resource",
							},
							VersionFrom:            &putPlan.ID,
							VersionedResourceTypes: resourceTypes,
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
				putPlan := expectedPlanFactory.NewPlan(atc.PutPlan{
					Type:     "git",
					Name:     "some-resource",
					Resource: "some-resource",
					Source: atc.Source{
						"uri": "git://some-resource",
					},
					Params:                 nil,
					VersionedResourceTypes: resourceTypes,
				})

				expectedPlan := expectedPlanFactory.NewPlan(atc.DoPlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "those who resist our will",
						VersionedResourceTypes: resourceTypes,
					}),
					expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
						Step: putPlan,
						Next: expectedPlanFactory.NewPlan(atc.GetPlan{
							Type:     "git",
							Name:     "some-resource",
							Resource: "some-resource",
							Source: atc.Source{
								"uri": "git://some-resource",
							},
							VersionFrom:            &putPlan.ID,
							VersionedResourceTypes: resourceTypes,
						}),
					}),
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "some-other-task",
						VersionedResourceTypes: resourceTypes,
					}),
				})

				actual, err := buildFactory.Create(input, resources, resourceTypes, nil)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(testhelpers.MatchPlan(expectedPlan))
			})
		})
	})
})
