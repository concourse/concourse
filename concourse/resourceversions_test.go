package concourse_test

import (
	"fmt"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Resource Versions", func() {
	Describe("ResourceVersions", func() {
		var (
			expectedVersions []atc.VersionedResource
			expectedURL      string
			expectedQuery    string
		)

		JustBeforeEach(func() {
			expectedVersions = []atc.VersionedResource{
				{
					Version: atc.Version{"version": "v1"},
				},
				{
					Version: atc.Version{"version": "v2"},
				},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL, expectedQuery),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedVersions),
				),
			)
		})

		Context("when since, until, and limit are 0", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/pipelines/mypipeline/resources/myresource/versions")
			})

			It("calls to get all versions", func() {
				versions, _, found, err := client.ResourceVersions("mypipeline", "myresource", concourse.Page{})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(Equal(expectedVersions))
			})
		})

		Context("when since is specified", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/pipelines/mypipeline/resources/myresource/versions")
				expectedQuery = fmt.Sprint("since=24")
			})

			It("calls to get all versions since that id", func() {
				versions, _, found, err := client.ResourceVersions("mypipeline", "myresource", concourse.Page{Since: 24})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(Equal(expectedVersions))
			})

			Context("and limit is specified", func() {
				BeforeEach(func() {
					expectedQuery = fmt.Sprint("since=24&limit=5")
				})

				It("appends limit to the url", func() {
					versions, _, found, err := client.ResourceVersions("mypipeline", "myresource", concourse.Page{Since: 24, Limit: 5})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(versions).To(Equal(expectedVersions))
				})
			})
		})

		Context("when until is specified", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/pipelines/mypipeline/resources/myresource/versions")
				expectedQuery = fmt.Sprint("until=26")
			})

			It("calls to get all versions until that id", func() {
				versions, _, found, err := client.ResourceVersions("mypipeline", "myresource", concourse.Page{Until: 26})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(Equal(expectedVersions))
			})

			Context("and limit is specified", func() {
				BeforeEach(func() {
					expectedQuery = fmt.Sprint("until=26&limit=15")
				})

				It("appends limit to the url", func() {
					versions, _, found, err := client.ResourceVersions("mypipeline", "myresource", concourse.Page{Until: 26, Limit: 15})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(versions).To(Equal(expectedVersions))
				})
			})
		})

		Context("when since and until are both specified", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/pipelines/mypipeline/resources/myresource/versions")
				expectedQuery = fmt.Sprint("until=26")
			})

			It("only sends the until", func() {
				versions, _, found, err := client.ResourceVersions("mypipeline", "myresource", concourse.Page{Since: 24, Until: 26})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(Equal(expectedVersions))
			})
		})

		Context("when the server returns an error", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/pipelines/mypipeline/resources/myresource/versions")

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusInternalServerError, ""),
					),
				)
			})

			It("returns false and an error", func() {
				_, _, found, err := client.ResourceVersions("mypipeline", "myresource", concourse.Page{})
				Expect(err).To(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when the server returns not found", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/pipelines/mypipeline/resources/myresource/versions")

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns false and no error", func() {
				_, _, found, err := client.ResourceVersions("mypipeline", "myresource", concourse.Page{})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("pagination data", func() {
			Context("with a link header", func() {
				BeforeEach(func() {
					expectedURL = fmt.Sprint("/api/v1/pipelines/mypipeline/resources/myresource/versions")

					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", expectedURL),
							ghttp.RespondWithJSONEncoded(http.StatusOK, expectedVersions, http.Header{
								"Link": []string{
									`<http://some-url.com/api/v1/pipelines/mypipeline/resources/myresource/versions?since=452&limit=123>; rel="previous"`,
									`<http://some-url.com/api/v1/pipelines/mypipeline/resources/myresource/versions?until=254&limit=456>; rel="next"`,
								},
							}),
						),
					)
				})

				It("returns the pagination data from the header", func() {
					_, pagination, _, err := client.ResourceVersions("mypipeline", "myresource", concourse.Page{})
					Expect(err).ToNot(HaveOccurred())
					Expect(pagination.Previous).To(Equal(&concourse.Page{Since: 452, Limit: 123}))
					Expect(pagination.Next).To(Equal(&concourse.Page{Until: 254, Limit: 456}))
				})
			})
		})

		Context("without a link header", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/pipelines/mypipeline/resources/myresource/versions")

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedVersions, http.Header{}),
					),
				)
			})

			It("returns pagination data with nil pages", func() {
				_, pagination, _, err := client.ResourceVersions("mypipeline", "myresource", concourse.Page{})
				Expect(err).ToNot(HaveOccurred())

				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(BeNil())
			})
		})
	})
})
