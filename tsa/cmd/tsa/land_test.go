package main_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Land", func() {
	var landErr error

	JustBeforeEach(func() {
		landErr = tsaClient.Land(context.TODO())
	})

	Context("when the worker is registered globally", func() {
		BeforeEach(func() {
			tsaClient.Worker.Team = ""
		})

		Context("when landing with a global key", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = globalKey
			})

			Context("when the ATC is working", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/workers/some-worker/land"),
						ghttp.RespondWith(200, nil, nil),
					))
				})

				It("sends a request to the ATC to land the worker", func() {
					Expect(landErr).ToNot(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})

			Context("when the ATC responds with a missing worker (404)", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/workers/some-worker/land"),
						ghttp.RespondWith(404, nil, nil),
					))
				})

				It("fails", func() {
					Eventually(tsaRunner.Buffer()).Should(gbytes.Say("404"))
					Expect(landErr).To(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})

			Context("when the ATC responds with an error", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/workers/some-worker/land"),
						ghttp.RespondWith(500, nil, nil),
					))
				})

				It("fails", func() {
					Eventually(tsaRunner.Buffer()).Should(gbytes.Say("500"))
					Expect(landErr).To(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})
		})

		Context("when landing with some team's key", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = teamKey
			})

			Context("when the ATC is working", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/workers/some-worker/land"),
						ghttp.RespondWith(200, nil, nil),
					))
				})

				It("fails", func() {
					Expect(landErr).To(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(0))
				})
			})
		})
	})

	Context("when the worker is registered for a team", func() {
		BeforeEach(func() {
			tsaClient.Worker.Team = "some-team"
		})

		Context("when landing with the team key", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = teamKey
			})

			Context("when the ATC is working", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/workers/some-worker/land"),
						ghttp.RespondWith(200, nil, nil),
					))
				})

				It("sends the request as the specified team", func() {
					Expect(landErr).ToNot(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})
		})

		Context("when landing with some other team's key", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = otherTeamKey
			})

			Context("when the ATC is working", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/workers/some-worker/land"),
						ghttp.RespondWith(200, nil, nil),
					))
				})

				It("fails", func() {
					Expect(landErr).To(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(0))
				})
			})
		})

		Context("when landing with a global key", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = globalKey
			})

			Context("when the ATC is working", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/workers/some-worker/land"),
						ghttp.RespondWith(200, nil, nil),
					))
				})

				It("sends the request as the specified team", func() {
					Expect(landErr).ToNot(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})
		})
	})
})
