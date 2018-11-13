package concourse_test

import (
	"net/http"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Resource", func() {
	Describe("Resource", func() {
		var expectedResource atc.Resource

		var resource atc.Resource
		var found bool
		var clientErr error

		BeforeEach(func() {
			expectedResource = atc.Resource{
				Name: "some-name",
			}
		})

		JustBeforeEach(func() {
			resource, found, clientErr = team.Resource("some-pipeline", "myresource")
		})

		Context("when the server returns the resource", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/some-team/pipelines/some-pipeline/resources/myresource"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedResource),
					),
				)
			})

			It("returns the resource", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(resource).To(Equal(expectedResource))
			})
		})

		Context("when the server returns a 404", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/some-team/pipelines/some-pipeline/resources/myresource"),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns false for found and a nil error", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when the server returns a 500", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/some-team/pipelines/some-pipeline/resources/myresource"),
						ghttp.RespondWith(http.StatusInternalServerError, ""),
					),
				)
			})

			It("returns false for found and an error", func() {
				Expect(clientErr).To(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})
})
