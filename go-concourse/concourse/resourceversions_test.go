package concourse_test

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Resource Versions", func() {
	Describe("ResourceVersions", func() {
		expectedURL := fmt.Sprint("/api/v1/teams/some-team/pipelines/mypipeline/resources/myresource/versions")

		var expectedVersions []atc.ResourceVersion

		var page concourse.Page
		var filter atc.Version

		var versions []atc.ResourceVersion
		var pagination concourse.Pagination
		var found bool
		var clientErr error

		BeforeEach(func() {
			page = concourse.Page{}

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
			versions, pagination, found, clientErr = team.ResourceVersions("mypipeline", "myresource", page, filter)
		})

		Context("when from, to, and limit are 0", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedVersions),
					),
				)
			})

			It("calls to get all versions", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(Equal(expectedVersions))
			})
		})

		Context("when from is specified", func() {
			BeforeEach(func() {
				page = concourse.Page{From: 24}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, "from=24"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedVersions),
					),
				)
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

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, "from=24&limit=5"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedVersions),
					),
				)
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

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, "to=26"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedVersions),
					),
				)
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

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, "to=26&limit=15"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedVersions),
					),
				)
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

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, "to=26&from=24"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedVersions),
					),
				)
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

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, "filter=some:value"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedVersions),
					),
				)
			})

			It("sends filters as url params", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(Equal(expectedVersions))
			})
		})

		Context("when the server returns an error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusInternalServerError, ""),
					),
				)
			})

			It("returns false and an error", func() {
				Expect(clientErr).To(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when the server returns not found", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
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
		)

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", expectedURL),
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
					disabled, err := team.DisableResourceVersion(pipelineName, resourceName, resourceVersionID)
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
					disabled, err := team.DisableResourceVersion(pipelineName, resourceName, resourceVersionID)
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
					disabled, err := team.DisableResourceVersion(pipelineName, resourceName, resourceVersionID)
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
		)

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", expectedURL),
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
					enabled, err := team.EnableResourceVersion(pipelineName, resourceName, resourceVersionID)
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
					enabled, err := team.EnableResourceVersion(pipelineName, resourceName, resourceVersionID)
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
					enabled, err := team.EnableResourceVersion(pipelineName, resourceName, resourceVersionID)
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
		)

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", expectedURL),
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
					pinned, err := team.PinResourceVersion(pipelineName, resourceName, resourceVersionID)
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
					pinned, err := team.PinResourceVersion(pipelineName, resourceName, resourceVersionID)
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
					pinned, err := team.PinResourceVersion(pipelineName, resourceName, resourceVersionID)
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
		)

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", expectedURL),
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
					pinned, err := team.UnpinResource(pipelineName, resourceName)
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
					pinned, err := team.UnpinResource(pipelineName, resourceName)
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
					pinned, err := team.UnpinResource(pipelineName, resourceName)
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
		)

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", expectedURL),
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
						setComment, err := team.SetPinComment(pipelineName, resourceName, "some comment")
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
					setComment, err := team.SetPinComment(pipelineName, resourceName, "some comment")
					Expect(err).ToNot(HaveOccurred())
					Expect(setComment).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})
	})
})
