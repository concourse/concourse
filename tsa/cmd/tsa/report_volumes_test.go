package main_test

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ReportVolumes", func() {
	var reportErr error

	JustBeforeEach(func() {
		reportErr = tsaClient.ReportVolumes(context.TODO(), []string{"a", "b"})
	})

	Context("when the worker is registered globally", func() {
		BeforeEach(func() {
			tsaClient.Worker.Team = ""
		})

		Context("with a global key", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = globalKey
			})

			Context("when the ATC is working", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/volumes/report", "worker_name=some-worker"),
						ghttp.VerifyJSONRepresenting([]string{"a", "b"}),
						ghttp.RespondWith(http.StatusNoContent, ""),
					))
				})

				It("sends the correct request to the ATC", func() {
					Expect(reportErr).ToNot(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})

			Context("when the ATC responds with an error", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/volumes/report", "worker_name=some-worker"),
						ghttp.RespondWith(500, nil, nil),
					))
				})

				It("fails", func() {
					Eventually(tsaRunner.Buffer()).Should(gbytes.Say("500"))
					Expect(reportErr).To(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})
		})

		Context("with some team's key", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = teamKey
			})

			Context("when the ATC is working", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/volumes/report", "worker_name=some-worker"),
						ghttp.VerifyJSONRepresenting([]string{"a", "b"}),
						ghttp.RespondWith(http.StatusNoContent, ""),
					))
				})

				It("fails", func() {
					Expect(reportErr).To(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(0))
				})
			})
		})
	})

	Context("when the worker is registered for a team", func() {
		BeforeEach(func() {
			tsaClient.Worker.Team = "some-team"
		})

		Context("with the team key", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = teamKey
			})

			Context("when the ATC is working", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/volumes/report", "worker_name=some-worker"),
						ghttp.VerifyJSONRepresenting([]string{"a", "b"}),
						ghttp.RespondWith(http.StatusNoContent, ""),
					))
				})

				It("sends the request with a system token", func() {
					Expect(reportErr).ToNot(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})
		})

		Context("with some other team's key", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = otherTeamKey
			})

			Context("when the ATC is working", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/volumes/report", "worker_name=some-worker"),
						ghttp.VerifyJSONRepresenting([]string{"a", "b"}),
						ghttp.RespondWith(http.StatusNoContent, ""),
					))
				})

				It("fails", func() {
					Expect(reportErr).To(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(0))
				})
			})
		})

		Context("with a global key", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = globalKey
			})

			Context("when the ATC is working", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/volumes/report", "worker_name=some-worker"),
						ghttp.VerifyJSONRepresenting([]string{"a", "b"}),
						ghttp.RespondWith(http.StatusNoContent, ""),
					))
				})

				It("sends the request with a system token", func() {
					Expect(reportErr).ToNot(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})
		})
	})
})
