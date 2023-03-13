package concourse_test

import (
	"net/http"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Resource Type", func() {
	Describe("ListSharedForResourceType", func() {
		var (
			expectedShared atc.ResourcesAndTypes
			shared         atc.ResourcesAndTypes
			clientErr      error
			found          bool

			expectedURL   = "/api/v1/teams/some-team/pipelines/some-pipeline/resource-types/myresourcetype/shared"
			expectedQuery = "vars.branch=%22master%22"
			pipelineRef   = atc.PipelineRef{Name: "some-pipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}
		)

		BeforeEach(func() {
			expectedShared = atc.ResourcesAndTypes{
				ResourceTypes: atc.ResourceIdentifiers{
					{
						Name:         "myresourcetype",
						PipelineName: "some-pipeline",
						TeamName:     "some-team",
					},
				},
			}
		})

		JustBeforeEach(func() {
			shared, found, clientErr = team.ListSharedForResourceType(pipelineRef, "myresourcetype")
		})

		Context("when the server returns the shared resources and resource types", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, expectedQuery),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedShared),
					),
				)
			})

			It("returns the shared resources/resource-types", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(shared).To(Equal(expectedShared))
				Expect(found).To(BeTrue())
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

			It("returns a nil error and not found", func() {
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

			It("returns an error and not found", func() {
				Expect(clientErr).To(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})
})
