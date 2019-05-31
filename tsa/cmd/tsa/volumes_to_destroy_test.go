package main_test

import (
	"context"
	"net/http"

	"github.com/concourse/concourse/v5/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("VolumesToDestroy", func() {
	var handles []string
	var volumesErr error

	JustBeforeEach(func() {
		handles, volumesErr = tsaClient.VolumesToDestroy(context.TODO())
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
						ghttp.VerifyRequest("GET", "/api/v1/volumes/destroying", "worker_name=some-worker"),
						http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
							accessor := accessFactory.Create(r, atc.ListDestroyingVolumes)
							Expect(accessor.IsAuthenticated()).To(BeTrue())
							Expect(accessor.IsSystem()).To(BeTrue())
						}),
						ghttp.RespondWithJSONEncoded(200, []string{"a", "b"}),
					))
				})

				It("sends a request to the ATC to land the worker", func() {
					Expect(volumesErr).ToNot(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})

				It("returns the handles to destroy", func() {
					Expect(handles).To(Equal([]string{"a", "b"}))
				})
			})

			Context("when the ATC responds with an error", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/volumes/destroying", "worker_name=some-worker"),
						ghttp.RespondWith(500, nil, nil),
					))
				})

				It("fails", func() {
					Eventually(tsaRunner.Buffer()).Should(gbytes.Say("500"))
					Expect(volumesErr).To(HaveOccurred())
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
						ghttp.VerifyRequest("GET", "/api/v1/volumes/destroying", "worker_name=some-worker"),
						http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
							accessor := accessFactory.Create(r, atc.LandWorker)
							Expect(accessor.IsAuthenticated()).To(BeTrue())
							Expect(accessor.IsSystem()).To(BeTrue())
						}),
						ghttp.RespondWithJSONEncoded(200, []string{"a", "b"}),
					))
				})

				It("fails", func() {
					Expect(volumesErr).To(HaveOccurred())
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
						ghttp.VerifyRequest("GET", "/api/v1/volumes/destroying", "worker_name=some-worker"),
						http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
							accessor := accessFactory.Create(r, atc.LandWorker)
							Expect(accessor.IsAuthenticated()).To(BeTrue())
							Expect(accessor.IsSystem()).To(BeTrue())
						}),
						ghttp.RespondWithJSONEncoded(200, []string{"a", "b"}),
					))
				})

				It("sends the request with a system token", func() {
					Expect(volumesErr).ToNot(HaveOccurred())
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
						ghttp.VerifyRequest("GET", "/api/v1/volumes/destroying", "worker_name=some-worker"),
						http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
							accessor := accessFactory.Create(r, atc.LandWorker)
							Expect(accessor.IsAuthenticated()).To(BeTrue())
							Expect(accessor.IsSystem()).To(BeTrue())
						}),
						ghttp.RespondWithJSONEncoded(200, []string{"a", "b"}),
					))
				})

				It("fails", func() {
					Expect(volumesErr).To(HaveOccurred())
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
						ghttp.VerifyRequest("GET", "/api/v1/volumes/destroying", "worker_name=some-worker"),
						http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
							accessor := accessFactory.Create(r, atc.LandWorker)
							Expect(accessor.IsAuthenticated()).To(BeTrue())
							Expect(accessor.IsSystem()).To(BeTrue())
						}),
						ghttp.RespondWithJSONEncoded(200, []string{"a", "b"}),
					))
				})

				It("sends the request with a system token", func() {
					Expect(volumesErr).ToNot(HaveOccurred())
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})

				It("returns the handles to destroy", func() {
					Expect(handles).To(Equal([]string{"a", "b"}))
				})
			})
		})
	})
})
