package concourse_test

import (
	"net/http"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Resource", func() {
	Describe("team.ListResources", func() {
		var (
			expectedResources []atc.Resource

			expectedURL   = "/api/v1/teams/some-team/pipelines/some-pipeline/resources"
			expectedQuery = "instance_vars=%7B%22branch%22%3A%22master%22%7D"
			pipelineRef   = atc.PipelineRef{Name: "some-pipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}
		)

		BeforeEach(func() {
			expectedResources = []atc.Resource{
				{
					Name: "resource-1",
					Type: "type-1",
				},
				{
					Name: "resource-2",
					Type: "type-2",
				},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL, expectedQuery),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedResources),
				),
			)
		})

		It("returns resources that belong to the pipeline", func() {
			pipelines, err := team.ListResources(pipelineRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelines).To(Equal(expectedResources))
		})
	})

	Describe("Resource", func() {
		var (
			expectedResource atc.Resource
			resource         atc.Resource
			found            bool
			clientErr        error

			expectedURL   = "/api/v1/teams/some-team/pipelines/some-pipeline/resources/myresource"
			expectedQuery = "instance_vars=%7B%22branch%22%3A%22master%22%7D"
			pipelineRef   = atc.PipelineRef{Name: "some-pipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}
		)

		BeforeEach(func() {
			expectedResource = atc.Resource{
				Name: "some-name",
			}
		})

		JustBeforeEach(func() {
			resource, found, clientErr = team.Resource(pipelineRef, "myresource")
		})

		Context("when the server returns the resource", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, expectedQuery),
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
						ghttp.VerifyRequest("GET", expectedURL, expectedQuery),
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
						ghttp.VerifyRequest("GET", expectedURL, expectedQuery),
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
