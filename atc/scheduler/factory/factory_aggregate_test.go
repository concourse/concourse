package factory_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/scheduler/factory"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Aggregate", func() {
	var (
		buildFactory factory.BuildFactory

		resources           atc.ResourceConfigs
		resourceTypes       atc.VersionedResourceTypes
		actualPlanFactory   atc.PlanFactory
		expectedPlanFactory atc.PlanFactory

		fakeJob *dbfakes.FakeJob
	)

	BeforeEach(func() {
		actualPlanFactory = atc.NewPlanFactory(123)
		expectedPlanFactory = atc.NewPlanFactory(123)

		buildFactory = factory.NewBuildFactory(actualPlanFactory)

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

		fakeJob = new(dbfakes.FakeJob)
	})

	Context("when I have one aggregate", func() {
		BeforeEach(func() {
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
							},
							{
								Task: "some other thing",
							},
						},
					},
				},
			})
		})
		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(fakeJob, resources, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.AggregatePlan{
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "some thing",
					VersionedResourceTypes: resourceTypes,
				}),
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "some other thing",
					VersionedResourceTypes: resourceTypes,
				}),
			})
			Expect(actual).To(Equal(expected))
		})
	})

	Context("when I have nested aggregates", func() {
		BeforeEach(func() {
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
							},
							{
								Aggregate: &atc.PlanSequence{
									{
										Task: "some nested thing",
									},
									{
										Task: "some nested other thing",
									},
								},
							},
						},
					},
				},
			})
		})
		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(fakeJob, resources, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.AggregatePlan{
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "some thing",
					VersionedResourceTypes: resourceTypes,
				}),
				expectedPlanFactory.NewPlan(atc.AggregatePlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "some nested thing",
						VersionedResourceTypes: resourceTypes,
					}),
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "some nested other thing",
						VersionedResourceTypes: resourceTypes,
					}),
				}),
			})
			Expect(actual).To(Equal(expected))
		})
	})

	Context("when I have an aggregate with hooks", func() {
		BeforeEach(func() {
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
								Success: &atc.PlanConfig{
									Task: "some success hook",
								},
							},
						},
					},
				},
			})
		})
		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(fakeJob, resources, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.AggregatePlan{
				expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
					Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "some thing",
						VersionedResourceTypes: resourceTypes,
					}),
					Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "some success hook",
						VersionedResourceTypes: resourceTypes,
					}),
				}),
			})
			Expect(actual).To(Equal(expected))
		})
	})

	Context("when I have a hook on an aggregate", func() {
		BeforeEach(func() {
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
							},
						},
						Success: &atc.PlanConfig{
							Task: "some success hook",
						},
					},
				},
			})
		})
		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(fakeJob, resources, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
				Step: expectedPlanFactory.NewPlan(atc.AggregatePlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "some thing",
						VersionedResourceTypes: resourceTypes,
					}),
				}),
				Next: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "some success hook",
					VersionedResourceTypes: resourceTypes,
				}),
			})
			Expect(actual).To(Equal(expected))
		})
	})
})
