package getresource_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/web/getresource"
	"github.com/concourse/go-concourse/concourse"
	cfakes "github.com/concourse/go-concourse/concourse/concoursefakes"
)

var _ = Describe("FetchTemplateData", func() {
	var fakeTeam *cfakes.FakeTeam
	var templateData TemplateData
	var fetchErr error

	BeforeEach(func() {
		fakeTeam = new(cfakes.FakeTeam)
	})

	JustBeforeEach(func() {
		templateData, fetchErr = FetchTemplateData("some-pipeline", "some-resource", fakeTeam, concourse.Page{
			Since: 398,
			Until: 2,
		})
	})

	It("calls to get the pipeline config", func() {
		Expect(fakeTeam.PipelineCallCount()).To(Equal(1))
		Expect(fakeTeam.PipelineArgsForCall(0)).To(Equal("some-pipeline"))
	})

	Context("when getting the pipeline returns an error", func() {
		var expectedErr error

		BeforeEach(func() {
			expectedErr = errors.New("disaster")
			fakeTeam.PipelineReturns(atc.Pipeline{}, false, expectedErr)
		})

		It("returns an error if the config could not be loaded", func() {
			Expect(fetchErr).To(Equal(expectedErr))
		})
	})

	Context("when the pipeline is not found", func() {
		BeforeEach(func() {
			fakeTeam.PipelineReturns(atc.Pipeline{}, false, nil)
		})

		It("returns an error if the config could not be loaded", func() {
			Expect(fetchErr).To(Equal(ErrConfigNotFound))
		})
	})

	Context("when the api returns the pipeline", func() {
		BeforeEach(func() {
			fakeTeam.PipelineReturns(atc.Pipeline{
				Groups: atc.GroupConfigs{
					{
						Name:      "group-with-resource",
						Resources: []string{"some-resource"},
					},
					{
						Name:      "group-without-resource",
						Resources: []string{"some-other-resource"},
					},
				},
			}, true, nil)
		})

		It("calls to get the resource", func() {
			Expect(fakeTeam.ResourceCallCount()).To(Equal(1))
			pipelineName, resourceName := fakeTeam.ResourceArgsForCall(0)
			Expect(pipelineName).To(Equal("some-pipeline"))
			Expect(resourceName).To(Equal("some-resource"))
		})

		Context("when the call returns an error", func() {
			var expectedErr error
			BeforeEach(func() {
				expectedErr = errors.New("nope")
				fakeTeam.ResourceReturns(atc.Resource{}, false, expectedErr)
			})

			It("errors", func() {
				Expect(fetchErr).To(Equal(expectedErr))
			})
		})

		Context("when it can't find the resource", func() {
			BeforeEach(func() {
				fakeTeam.ResourceReturns(atc.Resource{}, false, nil)
			})

			It("returns a resource not found error", func() {
				Expect(fetchErr).To(Equal(ErrResourceNotFound))
			})
		})
	})
})
