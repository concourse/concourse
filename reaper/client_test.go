package reaper_test

import (
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
			It("generate correct requests for all container handles", func() {
				expectedBody, _ := json.Marshal([]string{"handle-1", "handle-2"})

				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/containers/destroy"),
						ghttp.VerifyBody(expectedBody),
						ghttp.RespondWithJSONEncoded(http.StatusNoContent, nil),
					),
				)
				err := client.DestroyContainers([]string{"handle-1", "handle-2"})
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns proper error response", func() {
				expectedBody, _ := json.Marshal([]string{"handle-1", "handle-2"})

				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/containers/destroy"),
						ghttp.VerifyBody(expectedBody),
						ghttp.RespondWithJSONEncoded(http.StatusInternalServerError, nil),
					),
				)

				err := client.DestroyContainers([]string{"handle-1", "handle-2"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("failed-to-destroy-containers"))
			})
		})

		AfterEach(func() {
			fakeServer.Close()
		})
	})
})
