package getjob_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"

	. "github.com/concourse/atc/web/getjob"
	"github.com/concourse/atc/web/group"

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
		templateData, fetchErr = FetchTemplateData("some-pipeline", fakeTeam, "some-job", concourse.Page{
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
						Name: "group-with-job",
						Jobs: []string{"some-job"},
					},
					{
						Name: "group-without-job",
						Jobs: []string{"some-other-job"},
					},
				},
			}, true, nil)
		})

		It("calls to get the job from the client", func() {
			actualPipelineName, actualJobName := fakeTeam.JobArgsForCall(0)
			Expect(actualPipelineName).To(Equal("some-pipeline"))
			Expect(actualJobName).To(Equal("some-job"))
		})

		Context("when the client returns a job", func() {
			BeforeEach(func() {
				fakeTeam.JobReturns(atc.Job{}, true, nil)
				fakeTeam.NameReturns("some-team")
			})

			It("returns the correct TemplateData", func() {
				Expect(templateData.TeamName).To(Equal("some-team"))
				Expect(templateData.PipelineName).To(Equal("some-pipeline"))
				Expect(templateData.JobName).To(Equal("some-job"))
				Expect(templateData.Since).To(Equal(398))
				Expect(templateData.Until).To(Equal(2))

				Expect(templateData.GroupStates).To(ConsistOf([]group.State{
					{
						Name:    "group-with-job",
						Enabled: true,
					},
					{
						Name:    "group-without-job",
						Enabled: false,
					},
				}))
			})
		})

		Context("when the client returns an error", func() {
			var expectedErr error
			BeforeEach(func() {
				expectedErr = errors.New("nope")
				fakeTeam.JobReturns(atc.Job{}, false, expectedErr)
			})

			It("returns an error", func() {
				Expect(fetchErr).To(Equal(expectedErr))
			})
		})

		Context("when the job could not be found", func() {
			BeforeEach(func() {
				fakeTeam.JobReturns(atc.Job{}, false, nil)
			})

			It("returns an error", func() {
				Expect(fetchErr).To(Equal(ErrJobConfigNotFound))
			})
		})
	})
})
