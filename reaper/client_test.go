package reaper_test

import (
	"bytes"
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/worker/reaper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Client", func() {

	Describe("It generates correct requests", func() {
		var (
			client     ReaperClient
			fakeServer *ghttp.Server
			logger     lager.Logger
		)

		BeforeEach(func() {
			fakeServer = ghttp.NewServer()
			logger = lagertest.NewTestLogger("reaper-client")
			client = NewClient(fakeServer.URL(), logger)
		})

		Describe("ping request", func() {
			It("generate correct request", func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/ping"),
						ghttp.RespondWith(http.StatusOK, nil),
					),
				)

				err := client.Ping()
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns proper error response", func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/ping"),
						ghttp.RespondWith(http.StatusInternalServerError, nil),
					),
				)

				err := client.Ping()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Unable to reach garden"))
			})
		})

		Describe("destroy containers request", func() {
			var (
				buf     bytes.Buffer
				handles = []string{"handle-1", "handle-2"}
			)

			BeforeEach(func() {
				json.NewEncoder(&buf).Encode(handles)
			})

			It("generate correct requests for all container handles", func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/containers/destroy"),
						ghttp.VerifyJSON(buf.String()),
						ghttp.RespondWithJSONEncoded(http.StatusNoContent, nil),
					),
				)
				err := client.DestroyContainers(handles)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns proper error response", func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/containers/destroy"),
						ghttp.VerifyJSON(buf.String()),
						ghttp.RespondWithJSONEncoded(http.StatusInternalServerError, nil),
					),
				)

				err := client.DestroyContainers(handles)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("received-500-response"))
			})
		})

		Describe("list containers request", func() {
			var handles []string
			It("generate correct requests for all container handles", func() {
				handles = []string{"handle-1", "handle-2"}
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/list"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, &handles),
					),
				)
				respHandles, err := client.ListHandles()
				Expect(err).ToNot(HaveOccurred())
				Expect(respHandles).To(Equal(handles))
				Expect(len(fakeServer.ReceivedRequests())).To(Equal(1))
			})

			It("returns proper error response", func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/list"),
						ghttp.RespondWithJSONEncoded(http.StatusInternalServerError, nil),
					),
				)
				_, err := client.ListHandles()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("received-500-response"))
			})
		})

		AfterEach(func() {
			fakeServer.Close()
		})
	})
})
