package main_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ContainersToDestroy", func() {
	var handles []string
	var containersErr error

	JustBeforeEach(func() {
		handles, containersErr = tsaClient.ContainersToDestroy(context.TODO())
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
						ghttp.VerifyRequest("GET", "/api/v1/containers/destroying", "worker_name=some-worker"),
						ghttp.RespondWithJSONEncoded(200, []string{"a", "b"}),
					))
				})

				It("sends a request to the ATC to land the worker", func() {
					Expect(containersErr).ToNot(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})

				It("returns the handles to destroy", func() {
					Expect(handles).To(Equal([]string{"a", "b"}))
				})
			})

			Context("when the ATC responds with an error", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/containers/destroying", "worker_name=some-worker"),
						ghttp.RespondWith(500, nil, nil),
					))
				})

				It("fails", func() {
					Eventually(tsaRunner.Buffer()).Should(gbytes.Say("500"))
					Expect(containersErr).To(HaveOccurred())
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
						ghttp.VerifyRequest("GET", "/api/v1/containers/destroying", "worker_name=some-worker"),
						ghttp.RespondWithJSONEncoded(200, []string{"a", "b"}),
					))
				})

				It("fails", func() {
					Expect(containersErr).To(HaveOccurred())
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
						ghttp.VerifyRequest("GET", "/api/v1/containers/destroying", "worker_name=some-worker"),
						ghttp.RespondWithJSONEncoded(200, []string{"a", "b"}),
					))
				})

				It("sends the request with a system token", func() {
					Expect(containersErr).ToNot(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})

				It("returns the handles to destroy", func() {
					Expect(handles).To(Equal([]string{"a", "b"}))
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
						ghttp.VerifyRequest("GET", "/api/v1/containers/destroying", "worker_name=some-worker"),
						ghttp.RespondWithJSONEncoded(200, []string{"a", "b"}),
					))
				})

				It("fails", func() {
					Expect(containersErr).To(HaveOccurred())
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
						ghttp.VerifyRequest("GET", "/api/v1/containers/destroying", "worker_name=some-worker"),
						ghttp.RespondWithJSONEncoded(200, []string{"a", "b"}),
					))
				})

				It("sends the request with a system token", func() {
					Expect(containersErr).ToNot(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})

				It("returns the handles to destroy", func() {
					Expect(handles).To(Equal([]string{"a", "b"}))
				})
			})
		})
	})
})
