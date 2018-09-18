package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"
	"github.com/concourse/atc/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Task", func() {
	Describe("TaskPlan", func() {
		var (
			buildFactory factory.BuildFactory

			resources           atc.ResourceConfigs
			resourceTypes       atc.VersionedResourceTypes
			input               atc.JobConfig
			actualPlanFactory   atc.PlanFactory
			expectedPlanFactory atc.PlanFactory
			params              atc.Params
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
				params = atc.Params{
					"foo": "bar",
					"baz": "qux",
				}

				input = atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Task:   "some-task",
							Params: params,
						},
					},
				}
			})

			It("returns the correct plan", func() {
				actual, err := buildFactory.Create(input, resources, resourceTypes, nil)
				Expect(err).NotTo(HaveOccurred())

				expected := expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "some-task",
					VersionedResourceTypes: resourceTypes,
					Params:                 params,
				})
				Expect(actual).To(testhelpers.MatchPlan(expected))
			})
		})

		Context("when input mapping is specified", func() {
			BeforeEach(func() {
				input = atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Task: "some-task",
							InputMapping: map[string]string{
								"bosh-release": "concourse-release",
							},
							TaskConfig: &atc.TaskConfig{
								Inputs: []atc.TaskInputConfig{
									{
										Name: "bosh-release",
										Path: "fake-bosh-release-path",
									},
									{
										Name: "other-input",
										Path: "fake-other-input-path",
									},
								},
							},
						},
					},
				}
			})

			It("creates build plan with aliased inputs", func() {
				actual, err := buildFactory.Create(input, resources, resourceTypes, nil)
				Expect(err).NotTo(HaveOccurred())

				expected := expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "some-task",
					VersionedResourceTypes: resourceTypes,
					InputMapping: map[string]string{
						"bosh-release": "concourse-release",
					},
					Config: &atc.TaskConfig{
						Inputs: []atc.TaskInputConfig{
							{
								Name: "bosh-release",
								Path: "fake-bosh-release-path",
							},
							{
								Name: "other-input",
								Path: "fake-other-input-path",
							},
						},
					},
				})
				Expect(actual).To(testhelpers.MatchPlan(expected))
			})
		})

		Context("when output mapping is specified", func() {
			BeforeEach(func() {
				input = atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Task: "some-task",
							OutputMapping: map[string]string{
								"bosh-release": "concourse-release",
							},
							TaskConfig: &atc.TaskConfig{
								Outputs: []atc.TaskOutputConfig{
									{
										Name: "bosh-release",
										Path: "fake-bosh-release-path",
									},
									{
										Name: "other-input",
										Path: "fake-other-input-path",
									},
								},
							},
						},
					},
				}
			})

			It("creates build plan with aliased output", func() {
				actual, err := buildFactory.Create(input, resources, resourceTypes, nil)
				Expect(err).NotTo(HaveOccurred())

				expected := expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "some-task",
					VersionedResourceTypes: resourceTypes,
					OutputMapping: map[string]string{
						"bosh-release": "concourse-release",
					},
					Config: &atc.TaskConfig{
						Outputs: []atc.TaskOutputConfig{
							{
								Name: "bosh-release",
								Path: "fake-bosh-release-path",
							},
							{
								Name: "other-input",
								Path: "fake-other-input-path",
							},
						},
					},
				})
				Expect(actual).To(testhelpers.MatchPlan(expected))
			})
		})
	})
})
