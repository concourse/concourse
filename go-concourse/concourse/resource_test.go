package concourse_test

import (
	"net/http"
	"strings"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Resource", func() {
	Describe("team.ListResources", func() {
		var (
			expectedResources []atc.Resource

			expectedURL   = "/api/v1/teams/some-team/pipelines/some-pipeline/resources"
			expectedQuery = "vars.branch=%22master%22"
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
			expectedQuery = "vars.branch=%22master%22"
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

	Describe("ClearResourceCache", func() {
		var (
			expectedQueryParams []string
			expectedURL         = "/api/v1/teams/some-team/pipelines/some-pipeline/resources/some-resource/cache"
			version             = atc.Version{}
			pipelineRef         = atc.PipelineRef{Name: "some-pipeline"}
		)

		Context("when the API call succeeds", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL, strings.Join(expectedQueryParams, "&")),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.ClearResourceCacheResponse{CachesRemoved: 1}),
					),
				)
			})

			It("return no error", func() {
				rowsDeleted, err := team.ClearResourceCache(pipelineRef, "some-resource", version)
				Expect(err).NotTo(HaveOccurred())
				Eventually(rowsDeleted).Should(Equal(int64(1)))
			})
		})

		Context("when the API call errors", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL, strings.Join(expectedQueryParams, "&")),
						ghttp.RespondWithJSONEncoded(http.StatusInternalServerError, nil),
					),
				)
			})
			It("returns error", func() {
				_, err := team.ClearResourceCache(pipelineRef, "some-resource", version)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("ListSharedForResource", func() {
		var (
			expectedShared atc.ResourcesAndTypes
			shared         atc.ResourcesAndTypes
			clientErr      error
			found          bool

			expectedURL   = "/api/v1/teams/some-team/pipelines/some-pipeline/resources/myresource/shared"
			expectedQuery = "vars.branch=%22master%22"
			pipelineRef   = atc.PipelineRef{Name: "some-pipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}
		)

		BeforeEach(func() {
			expectedShared = atc.ResourcesAndTypes{
				Resources: atc.ResourceIdentifiers{
					{
						Name:         "myresource",
						PipelineName: "some-pipeline",
						TeamName:     "some-team",
					},
				},
			}
		})

		JustBeforeEach(func() {
			shared, found, clientErr = team.ListSharedForResource(pipelineRef, "myresource")
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
