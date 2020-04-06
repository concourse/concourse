package main_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Delete", func() {
	var deleteErr error

	JustBeforeEach(func() {
		deleteErr = tsaClient.Delete(context.TODO())
	})

	Context("when the worker is registered globally", func() {
		BeforeEach(func() {
			tsaClient.Worker.Team = ""
		})

		Context("when retiring with a global key", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = globalKey
			})

			Context("when the ATC is working", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/api/v1/workers/some-worker"),
						ghttp.RespondWith(200, nil, nil),
					))
				})

				It("sends a request to the ATC to delete the worker", func() {
					Expect(deleteErr).ToNot(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})

			Context("when the ATC responds with a missing worker (404)", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/api/v1/workers/some-worker"),
						ghttp.RespondWith(404, nil, nil),
					))
				})

				It("fails", func() {
					Eventually(tsaRunner.Buffer()).Should(gbytes.Say("404"))
					Expect(deleteErr).To(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})

			Context("when the ATC responds with an error", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/api/v1/workers/some-worker"),
						ghttp.RespondWith(500, nil, nil),
					))
				})

				It("fails", func() {
					Eventually(tsaRunner.Buffer()).Should(gbytes.Say("500"))
					Expect(deleteErr).To(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})
		})

		Context("when retiring with some team's key", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = teamKey
			})

			Context("when the ATC is working", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/api/v1/workers/some-worker"),
						ghttp.RespondWith(200, nil, nil),
					))
				})

				It("fails", func() {
					Expect(deleteErr).To(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(0))
				})
			})
		})
	})

	Context("when the worker is registered for a team", func() {
		BeforeEach(func() {
			tsaClient.Worker.Team = "some-team"
		})

		Context("when retiring with the team key", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = teamKey
			})

			Context("when the ATC is working", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/api/v1/workers/some-worker"),
						ghttp.RespondWith(200, nil, nil),
					))
				})

				It("sends the request as the specified team", func() {
					Expect(deleteErr).ToNot(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})
		})

		Context("when retiring with some other team's key", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = otherTeamKey
			})

			Context("when the ATC is working", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/api/v1/workers/some-worker"),
						ghttp.RespondWith(200, nil, nil),
					))
				})

				It("fails", func() {
					Expect(deleteErr).To(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(0))
				})
			})
		})

		Context("when retiring with a global key", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = globalKey
			})

			Context("when the ATC is working", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/api/v1/workers/some-worker"),
						ghttp.RespondWith(200, nil, nil),
					))
				})

				It("sends the request as the specified team", func() {
					Expect(deleteErr).ToNot(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})
		})
	})
})
