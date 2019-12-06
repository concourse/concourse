package factory_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/scheduler/factory"
	"github.com/concourse/concourse/atc/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Get", func() {
	var (
		buildFactory factory.BuildFactory

		resources           atc.ResourceConfigs
		resourceTypes       atc.VersionedResourceTypes
		fakeJob             *dbfakes.FakeJob
		actualPlanFactory   atc.PlanFactory
		expectedPlanFactory atc.PlanFactory
		version             atc.Version
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

	Context("with a get at the top-level", func() {
		BeforeEach(func() {
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Get:      "some-get",
						Resource: "some-resource",
					},
				},
			})
		})

		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(fakeJob, resources, resourceTypes, nil)
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
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Get:      "some-get",
						Resource: "not-a-resource",
					},
				},
			})
		})

		It("returns the correct error", func() {
			_, err := buildFactory.Create(fakeJob, resources, resourceTypes, nil)
			Expect(err).To(Equal(factory.ErrResourceNotFound))
		})
	})
})
