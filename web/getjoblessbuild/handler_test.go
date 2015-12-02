package getjoblessbuild_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/web/getjoblessbuild"
	cfakes "github.com/concourse/go-concourse/concourse/fakes"
)

var _ = Describe("Handler", func() {
	Describe("creating the Template Data", func() {
		var (
			fakeClient   *cfakes.FakeClient
			fetchErr     error
			templateData TemplateData
		)

		BeforeEach(func() {
			fakeClient = new(cfakes.FakeClient)
		})

		JustBeforeEach(func() {
			templateData, fetchErr = FetchTemplateData("3", fakeClient)
		})

		It("uses the client to get the build", func() {
			Expect(fakeClient.BuildCallCount()).To(Equal(1))
			Expect(fakeClient.BuildArgsForCall(0)).To(Equal("3"))
		})

		Context("when the client returns an error", func() {
			var expectedError error

			BeforeEach(func() {
				expectedError = errors.New("NOOOOOOO")
				fakeClient.BuildReturns(atc.Build{}, false, expectedError)
			})

			It("returns the error", func() {
				Expect(fetchErr).To(HaveOccurred())
				Expect(fetchErr).To(Equal(expectedError))
			})
		})

		Context("when the client returns not found", func() {
			BeforeEach(func() {
				fakeClient.BuildReturns(atc.Build{}, false, nil)
			})

			It("returns the error", func() {
				Expect(fetchErr).To(HaveOccurred())
				Expect(fetchErr).To(Equal(ErrBuildNotFound))
			})
		})

		Context("when the client returns a build", func() {
			var expectedBuild atc.Build

			BeforeEach(func() {
				expectedBuild = atc.Build{
					ID: 2,
				}
				fakeClient.BuildReturns(expectedBuild, true, nil)
			})

			It("returns the build in the template data", func() {
				Expect(templateData.Build).To(Equal(expectedBuild))
			})
		})
	})
})
