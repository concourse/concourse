package concourse_test

import (
	"net/http"

	"github.com/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Resource", func() {
	Describe("Resource", func() {
		var (
			expectedResource atc.Resource
			serverStatusCode int
		)

		BeforeEach(func() {
			expectedResource = atc.Resource{
				Name: "some-name",
			}
			serverStatusCode = http.StatusOK
		})

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/pipelines/some-pipeline/resources/myresource"),
					ghttp.RespondWithJSONEncoded(serverStatusCode, expectedResource),
				),
			)
		})

		It("returns resources", func() {
			resource, found, err := client.Resource("some-pipeline", "myresource")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource).To(Equal(expectedResource))
		})

		Context("when the server returns a 404", func() {
			BeforeEach(func() {
				serverStatusCode = http.StatusNotFound
			})

			It("returns false for found and a nil error", func() {
				_, found, err := client.Resource("some-pipeline", "myresource")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when the server returns a 500", func() {
			BeforeEach(func() {
				serverStatusCode = http.StatusInternalServerError
			})

			It("returns false for found and an error", func() {
				_, found, err := client.Resource("some-pipeline", "myresource")
				Expect(err).To(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})
})
