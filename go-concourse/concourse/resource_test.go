package concourse_test

import (
	"fmt"
	"net/http"

	"github.com/concourse/atc"
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

	Describe("PauseResource", func() {
		var (
			expectedStatus int
			pipelineName   = "banana"
			resourceName   = "disResource"
			expectedURL    = fmt.Sprintf("/api/v1/teams/some-team/pipelines/%s/resources/%s/pause", pipelineName, resourceName)
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

			It("calls the pause resource and returns no error", func() {
				Expect(func() {
					paused, err := team.PauseResource(pipelineName, resourceName)
					Expect(err).NotTo(HaveOccurred())
					Expect(paused).To(BeTrue())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the pause resource call fails", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusInternalServerError
			})

			It("calls the pause resource and returns an error", func() {
				Expect(func() {
					paused, err := team.PauseResource(pipelineName, resourceName)
					Expect(err).To(HaveOccurred())
					Expect(paused).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the resource does not exist", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusNotFound
			})

			It("calls the pause resource and returns an error", func() {
				Expect(func() {
					paused, err := team.PauseResource(pipelineName, resourceName)
					Expect(err).ToNot(HaveOccurred())
					Expect(paused).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})
	})

	Describe("UnpauseResource", func() {
		var (
			expectedStatus int
			pipelineName   = "banana"
			resourceName   = "disResource"
			expectedURL    = fmt.Sprintf("/api/v1/teams/some-team/pipelines/%s/resources/%s/unpause", pipelineName, resourceName)
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

			It("calls the unpause resource and returns no error", func() {
				Expect(func() {
					paused, err := team.UnpauseResource(pipelineName, resourceName)
					Expect(err).NotTo(HaveOccurred())
					Expect(paused).To(BeTrue())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the unpause resource call fails", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusInternalServerError
			})

			It("calls the unpause resource and returns an error", func() {
				Expect(func() {
					paused, err := team.UnpauseResource(pipelineName, resourceName)
					Expect(err).To(HaveOccurred())
					Expect(paused).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the resource does not exist", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusNotFound
			})

			It("calls the unpause resource and returns an error", func() {
				Expect(func() {
					paused, err := team.UnpauseResource(pipelineName, resourceName)
					Expect(err).ToNot(HaveOccurred())
					Expect(paused).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})
	})
})
