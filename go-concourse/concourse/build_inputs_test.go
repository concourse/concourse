package concourse_test

import (
	"net/http"

	"github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Build Inputs", func() {
	Describe("BuildInputsForJob", func() {
		expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/jobs/myjob/inputs"

		Context("when pipeline/job exists", func() {
			var expectedBuildInputs []atc.BuildInput

			BeforeEach(func() {
				expectedBuildInputs = []atc.BuildInput{
					{
						Name:     "myfirstinput",
						Resource: "myfirstinput",
					},
					{
						Name:     "mySecondinput",
						Resource: "mySecondinput",
					},
				}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuildInputs),
					),
				)
			})

			It("returns the input configuration for the given job", func() {
				buildInputs, found, err := team.BuildInputsForJob("mypipeline", "myjob")
				Expect(err).NotTo(HaveOccurred())
				Expect(buildInputs).To(Equal(expectedBuildInputs))
				Expect(found).To(BeTrue())
			})
		})

		Context("when pipeline/job does not exist", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns false in the found value and no error", func() {
				_, found, err := team.BuildInputsForJob("mypipeline", "myjob")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("BuildsWithVersionAsInput", func() {
		expectedURL := "/api/v1/teams/some-team/pipelines/some-pipeline/resources/myresource/versions/2/input_to"

		var expectedBuilds []atc.Build

		var actualBuilds []atc.Build
		var found bool
		var clientErr error

		BeforeEach(func() {
			expectedBuilds = []atc.Build{
				{
					ID:     2,
					Name:   "some-build",
					Status: "started",
				},
				{
					ID:     3,
					Name:   "some-other-build",
					Status: "started",
				},
			}
		})

		JustBeforeEach(func() {
			actualBuilds, found, clientErr = team.BuildsWithVersionAsInput("some-pipeline", "myresource", 2)
		})

		Context("when the server returns builds", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuilds),
					),
				)
			})

			It("returns the builds for a given resource_version_id", func() {
				Expect(clientErr).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(actualBuilds).To(Equal(expectedBuilds))
			})
		})

		Context("when the server returns a 404", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns found of false and no errr", func() {
				Expect(clientErr).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when the server returns a 500 error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusInternalServerError, ""),
					),
				)
			})

			It("returns found of false and no errr", func() {
				Expect(clientErr).To(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})
})
