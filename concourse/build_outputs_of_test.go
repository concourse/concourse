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
		var (
			expectedBuilds   []atc.Build
			serverStatusCode int
		)

		BeforeEach(func() {
			serverStatusCode = http.StatusOK
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
			expectedURL := "/api/v1/pipelines/some-pipeline/resources/myresource/versions/2/output_of"

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(serverStatusCode, expectedBuilds),
				),
			)
		})

		It("returns the builds for a given resource_version_id", func() {
			actualBuilds, found, err := client.BuildsWithVersionAsOutput("some-pipeline", "myresource", 2)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(actualBuilds).To(Equal(expectedBuilds))
		})

		Context("when the server returns a 404", func() {
			BeforeEach(func() {
				expectedBuilds = nil
				serverStatusCode = http.StatusNotFound
			})

			It("returns found of false and no errr", func() {
				_, found, err := client.BuildsWithVersionAsOutput("some-pipeline", "myresource", 2)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when the server returns a 500 error", func() {
			BeforeEach(func() {
				expectedBuilds = nil
				serverStatusCode = http.StatusInternalServerError
			})

			It("returns found of false and no errr", func() {
				_, found, err := client.BuildsWithVersionAsOutput("some-pipeline", "myresource", 2)
				Expect(err).To(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})
})
