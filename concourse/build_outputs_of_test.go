package concourse_test

import (
	"net/http"

	"github.com/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Build Outputs", func() {
	Describe("BuildsWithVersionAsOutput", func() {
		expectedURL := "/api/v1/teams/some-team/pipelines/some-pipeline/resources/myresource/versions/2/output_of"

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
			actualBuilds, found, clientErr = team.BuildsWithVersionAsOutput("some-pipeline", "myresource", 2)
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
