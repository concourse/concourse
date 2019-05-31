package concourse_test

import (
	"net/http"

	"github.com/concourse/concourse/v5/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Containers", func() {
	Describe("ListContainers", func() {
		Context("when passed an empty specification list", func() {
			var expectedContainers []atc.Container

			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/containers"

				expectedContainers = []atc.Container{
					{
						ID:               "myid-1",
						PipelineName:     "mypipeline-1",
						WorkingDirectory: "/tmp/build/some-guid",
					},
					{
						ID:               "myid-2",
						PipelineName:     "mypipeline-2",
						WorkingDirectory: "/tmp/build/some-other-guid",
					},
				}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedContainers),
					),
				)
			})

			It("returns all the containers", func() {
				containers, err := team.ListContainers(map[string]string{})
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(Equal(expectedContainers))
			})
		})

		Context("when passed a nonempty specification list", func() {
			var (
				expectedContainers []atc.Container
				expectedQueryList  map[string]string
			)

			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/containers"
				expectedQueryList = map[string]string{
					"query1": "value1",
				}

				expectedContainers = []atc.Container{
					{
						ID:               "myid-1",
						PipelineName:     "mypipeline-1",
						WorkingDirectory: "/tmp/build/some-guid",
					},
				}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, "query1=value1"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedContainers),
					),
				)
			})

			It("returns the specified containers", func() {
				containers, err := team.ListContainers(expectedQueryList)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(Equal(expectedContainers))
			})
		})
	})

	Describe("GetContainer", func() {
		Context("when passed a container handle", func() {
			var expectedContainer atc.Container
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/containers/myid-1"

				expectedContainer = atc.Container{
					ID:               "myid-1",
					PipelineName:     "mypipeline-1",
					WorkingDirectory: "/tmp/build/some-guid",
				}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedContainer),
					),
				)
			})
			It("should return the associated container", func() {
				containers, err := team.GetContainer("myid-1")
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(Equal(expectedContainer))
			})
		})
	})
})
