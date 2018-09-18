package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"
	"github.com/concourse/atc/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Get", func() {
	var (
		buildFactory factory.BuildFactory

		resources           atc.ResourceConfigs
		resourceTypes       atc.VersionedResourceTypes
		input               atc.JobConfig
		actualPlanFactory   atc.PlanFactory
		expectedPlanFactory atc.PlanFactory
		version             atc.Version
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

	Context("with a get at the top-level", func() {
		BeforeEach(func() {
			input = atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Get:      "some-get",
						Resource: "some-resource",
					},
				},
			}
		})

		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(input, resources, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.GetPlan{
				Type:     "git",
				Name:     "some-get",
				Resource: "some-resource",
				Source: atc.Source{
					"uri": "git://some-resource",
				},
				Version:                &version,
				VersionedResourceTypes: resourceTypes,
			})
			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})

	Context("with a get for a non-existent resource", func() {
		BeforeEach(func() {
			input = atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Get:      "some-get",
						Resource: "not-a-resource",
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
