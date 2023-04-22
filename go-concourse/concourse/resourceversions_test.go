package concourse_test

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Resource Versions", func() {
	Describe("ResourceVersions", func() {
		var (
			expectedURL      = "/api/v1/teams/some-team/pipelines/mypipeline/resources/myresource/versions"
			expectedQuery    []string
			expectedStatus   = http.StatusOK
			expectedVersions []atc.ResourceVersion
			pipelineRef      = atc.PipelineRef{Name: "mypipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}
		)

		var page concourse.Page
		var filter atc.Version

		var versions []atc.ResourceVersion
		var pagination concourse.Pagination
		var found bool
		var clientErr error

		BeforeEach(func() {
			page = concourse.Page{}
			filter = atc.Version{}
			expectedQuery = []string{"vars.branch=%22master%22"}

			expectedVersions = []atc.ResourceVersion{
				{
					Version: atc.Version{"version": "v1"},
				},
				{
					Version: atc.Version{"version": "v2"},
				},
			}

		})

		JustBeforeEach(func() {

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL, strings.Join(expectedQuery, "&")),
					ghttp.RespondWithJSONEncoded(expectedStatus, expectedVersions),
				),
			)

			versions, pagination, found, clientErr = team.ResourceVersions(pipelineRef, "myresource", page, filter)
		})

		Context("when from, to, and limit are 0", func() {
			It("calls to get all versions", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(Equal(expectedVersions))
			})
		})

		Context("when from is specified", func() {
			BeforeEach(func() {
				page = concourse.Page{From: 24}
				expectedQuery = append(expectedQuery, "from=24")
			})

			It("calls to get all versions from that id", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(Equal(expectedVersions))
			})
		})

		Context("when from and limit is specified", func() {
			BeforeEach(func() {
				page = concourse.Page{From: 24, Limit: 5}
				expectedQuery = append(expectedQuery, "from=24", "limit=5")
			})

			It("appends limit to the url", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(Equal(expectedVersions))
			})
		})

		Context("when to is specified", func() {
			BeforeEach(func() {
				page = concourse.Page{To: 26}
				expectedQuery = append(expectedQuery, "to=26")
			})

			It("calls to get all versions to that id", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(Equal(expectedVersions))
			})
		})

		Context("when to and limit is specified", func() {
			BeforeEach(func() {
				page = concourse.Page{To: 26, Limit: 15}
				expectedQuery = append(expectedQuery, "to=26", "limit=15")
			})

			It("appends limit to the url", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(Equal(expectedVersions))
			})
		})

		Context("when from and to are both specified", func() {
			BeforeEach(func() {
				page = concourse.Page{From: 24, To: 26}
				expectedQuery = append(expectedQuery, "to=26", "from=24")
			})

			It("sends both from and the to", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(Equal(expectedVersions))
			})
		})

		Context("when filter is specified", func() {
			BeforeEach(func() {
				filter = atc.Version{"some": "value"}
				expectedQuery = append(expectedQuery, "filter=some:value")
			})

			It("sends filters as url params", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(Equal(expectedVersions))
			})
		})

		Context("when the server returns an error", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusInternalServerError
			})

			It("returns false and an error", func() {
				Expect(clientErr).To(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when the server returns not found", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusNotFound
			})

			It("returns false and no error", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Describe("pagination data", func() {
			Context("with a link header", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", expectedURL),
							ghttp.RespondWithJSONEncoded(http.StatusOK, expectedVersions, http.Header{
								"Link": []string{
									`<http://some-url.com/api/v1/teams/some-team/pipelines/mypipeline/resources/myresource/versions?from=452&limit=123>; rel="previous"`,
									`<http://some-url.com/api/v1/teams/some-team/pipelines/mypipeline/resources/myresource/versions?to=254&limit=456>; rel="next"`,
								},
							}),
						),
					)
				})

				It("returns the pagination data from the header", func() {
					Expect(clientErr).ToNot(HaveOccurred())
					Expect(pagination.Previous).To(Equal(&concourse.Page{From: 452, Limit: 123}))
					Expect(pagination.Next).To(Equal(&concourse.Page{To: 254, Limit: 456}))
				})
			})
		})

		Context("without a link header", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedVersions, http.Header{}),
					),
				)
			})

			It("returns pagination data with nil pages", func() {
				Expect(clientErr).ToNot(HaveOccurred())

				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(BeNil())
			})
		})
	})

	Describe("DisableResourceVersion", func() {
		var (
			expectedStatus    int
			pipelineName      = "banana"
			resourceName      = "myresource"
			resourceVersionID = 42
			expectedURL       = fmt.Sprintf("/api/v1/teams/some-team/pipelines/%s/resources/%s/versions/%s/disable", pipelineName, resourceName, strconv.Itoa(resourceVersionID))
			expectedQuery     = "vars.branch=%22master%22"
			pipelineRef       = atc.PipelineRef{Name: pipelineName, InstanceVars: atc.InstanceVars{"branch": "master"}}
		)

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", expectedURL, expectedQuery),
					ghttp.RespondWith(expectedStatus, nil),
				),
			)
		})

		Context("when the resource exists and there are no issues", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusOK
			})

			It("calls the disable resource and returns no error", func() {
				Expect(func() {
					disabled, err := team.DisableResourceVersion(pipelineRef, resourceName, resourceVersionID)
					Expect(err).NotTo(HaveOccurred())
					Expect(disabled).To(BeTrue())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the disable resource call fails", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusInternalServerError
			})

			It("calls the disable resource and returns an error", func() {
				Expect(func() {
					disabled, err := team.DisableResourceVersion(pipelineRef, resourceName, resourceVersionID)
					Expect(err).To(HaveOccurred())
					Expect(disabled).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the resource does not exist", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusNotFound
			})

			It("calls the disable resource and returns an error", func() {
				Expect(func() {
					disabled, err := team.DisableResourceVersion(pipelineRef, resourceName, resourceVersionID)
					Expect(err).ToNot(HaveOccurred())
					Expect(disabled).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})
	})

	Describe("EnableResourceVersion", func() {
		var (
			expectedStatus    int
			pipelineName      = "banana"
			resourceName      = "myresource"
			resourceVersionID = 42
			expectedURL       = fmt.Sprintf("/api/v1/teams/some-team/pipelines/%s/resources/%s/versions/%s/enable", pipelineName, resourceName, strconv.Itoa(resourceVersionID))
			expectedQuery     = "vars.branch=%22master%22"
			pipelineRef       = atc.PipelineRef{Name: pipelineName, InstanceVars: atc.InstanceVars{"branch": "master"}}
		)

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", expectedURL, expectedQuery),
					ghttp.RespondWith(expectedStatus, nil),
				),
			)
		})

		Context("when the resource exists and there are no issues", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusOK
			})

			It("calls the enable resource and returns no error", func() {
				Expect(func() {
					enabled, err := team.EnableResourceVersion(pipelineRef, resourceName, resourceVersionID)
					Expect(err).NotTo(HaveOccurred())
					Expect(enabled).To(BeTrue())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the enable resource call fails", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusInternalServerError
			})

			It("calls the enable resource and returns an error", func() {
				Expect(func() {
					enabled, err := team.EnableResourceVersion(pipelineRef, resourceName, resourceVersionID)
					Expect(err).To(HaveOccurred())
					Expect(enabled).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the resource does not exist", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusNotFound
			})

			It("calls the enable resource and returns an error", func() {
				Expect(func() {
					enabled, err := team.EnableResourceVersion(pipelineRef, resourceName, resourceVersionID)
					Expect(err).ToNot(HaveOccurred())
					Expect(enabled).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})
	})

	Describe("PinResourceVersion", func() {
		var (
			expectedStatus    int
			expectedBody      []byte
			pipelineName      = "banana"
			resourceName      = "myresource"
			resourceVersionID = 42
			expectedURL       = fmt.Sprintf("/api/v1/teams/some-team/pipelines/%s/resources/%s/versions/%s/pin", pipelineName, resourceName, strconv.Itoa(resourceVersionID))
			expectedQuery     = "vars.branch=%22master%22"
			pipelineRef       = atc.PipelineRef{Name: pipelineName, InstanceVars: atc.InstanceVars{"branch": "master"}}
		)

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", expectedURL, expectedQuery),
					ghttp.VerifyBody(expectedBody),
					ghttp.RespondWith(expectedStatus, nil),
				),
			)
		})

		Context("When the resource exists and there are no issues", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusOK
			})

			It("calls the pin resource and returns no error", func() {
				Expect(func() {
					pinned, err := team.PinResourceVersion(pipelineRef, resourceName, resourceVersionID)
					Expect(err).ToNot(HaveOccurred())
					Expect(pinned).To(BeTrue())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the resource does not exist", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusNotFound
			})

			It("calls the pin resource and returns an error", func() {
				Expect(func() {
					pinned, err := team.PinResourceVersion(pipelineRef, resourceName, resourceVersionID)
					Expect(err).ToNot(HaveOccurred())
					Expect(pinned).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the pin resource call fails", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusInternalServerError
			})

			It("calls the pin resource and returns an error", func() {
				Expect(func() {
					pinned, err := team.PinResourceVersion(pipelineRef, resourceName, resourceVersionID)
					Expect(err).To(HaveOccurred())
					Expect(pinned).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})
	})

	Describe("UnpinResource", func() {
		var (
			expectedStatus int
			pipelineName   = "banana"
			resourceName   = "myresource"
			expectedURL    = fmt.Sprintf("/api/v1/teams/some-team/pipelines/%s/resources/%s/unpin", pipelineName, resourceName)
			expectedQuery  = "vars.branch=%22master%22"
			pipelineRef    = atc.PipelineRef{Name: pipelineName, InstanceVars: atc.InstanceVars{"branch": "master"}}
		)

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", expectedURL, expectedQuery),
					ghttp.RespondWith(expectedStatus, nil),
				),
			)
		})

		Context("When the resource exists and there are no issues", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusOK
			})

			It("calls the unpin resource and returns no error", func() {
				Expect(func() {
					pinned, err := team.UnpinResource(pipelineRef, resourceName)
					Expect(err).ToNot(HaveOccurred())
					Expect(pinned).To(BeTrue())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the resource does not exist", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusNotFound
			})

			It("calls the unpin resource and returns an error", func() {
				Expect(func() {
					pinned, err := team.UnpinResource(pipelineRef, resourceName)
					Expect(err).ToNot(HaveOccurred())
					Expect(pinned).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the unpin resource call fails", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusInternalServerError
			})

			It("calls the unpin resource and returns an error", func() {
				Expect(func() {
					pinned, err := team.UnpinResource(pipelineRef, resourceName)
					Expect(err).To(HaveOccurred())
					Expect(pinned).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})
	})

	Describe("SetPinComment", func() {
		var (
			expectedStatus     int
			expectedPinComment = atc.SetPinCommentRequestBody{PinComment: "some comment"}
			pipelineName       = "banana"
			resourceName       = "myresource"
			expectedURL        = fmt.Sprintf("/api/v1/teams/some-team/pipelines/%s/resources/%s/pin_comment", pipelineName, resourceName)
			expectedQuery      = "vars.branch=%22master%22"
			pipelineRef        = atc.PipelineRef{Name: pipelineName, InstanceVars: atc.InstanceVars{"branch": "master"}}
		)

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", expectedURL, expectedQuery),
					ghttp.VerifyJSONRepresenting(expectedPinComment),
					ghttp.RespondWith(expectedStatus, nil),
				),
			)
		})

		Context("When the resource exists and there are no issues", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusOK
			})

			Context("when setting pin comment", func() {
				BeforeEach(func() {
				})

				It("calls set pin comment and returns no error", func() {
					Expect(func() {
						setComment, err := team.SetPinComment(pipelineRef, resourceName, "some comment")
						Expect(err).ToNot(HaveOccurred())
						Expect(setComment).To(BeTrue())
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(1))
				})
			})
		})

		Context("when the resource does not exist", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusNotFound
			})

			It("calls the pin comment and returns an error", func() {
				Expect(func() {
					setComment, err := team.SetPinComment(pipelineRef, resourceName, "some comment")
					Expect(err).ToNot(HaveOccurred())
					Expect(setComment).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})
	})

	Describe("ClearResourceVersions", func() {
		var (
			expectedQueryParams []string
			expectedURL         = "/api/v1/teams/some-team/pipelines/some-pipeline/resources/some-resource/versions"
			pipelineRef         = atc.PipelineRef{Name: "some-pipeline"}
		)

		Context("when the API call succeeds", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL, strings.Join(expectedQueryParams, "&")),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.ClearVersionsResponse{VersionsRemoved: 1}),
					),
				)
			})

			It("return no error", func() {
				rowsDeleted, err := team.ClearResourceVersions(pipelineRef, "some-resource")
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
				_, err := team.ClearResourceVersions(pipelineRef, "some-resource")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("ClearResourceTypeVersions", func() {
		var (
			expectedQueryParams []string
			expectedURL         = "/api/v1/teams/some-team/pipelines/some-pipeline/resource-types/some-resource-type/versions"
			pipelineRef         = atc.PipelineRef{Name: "some-pipeline"}
		)

		Context("when the API call succeeds", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL, strings.Join(expectedQueryParams, "&")),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.ClearVersionsResponse{VersionsRemoved: 1}),
					),
				)
			})

			It("return no error", func() {
				rowsDeleted, err := team.ClearResourceTypeVersions(pipelineRef, "some-resource-type")
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
				_, err := team.ClearResourceTypeVersions(pipelineRef, "some-resource-type")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
