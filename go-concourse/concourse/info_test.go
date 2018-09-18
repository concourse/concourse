package concourse_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc"
)

var _ = Describe("ATC Info", func() {
	Describe("GetInfo", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/info"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Info{
						Version: "12.3.4",
					}),
				),
			)
		})

		It("returns the version that was returned from the server", func() {
			info, err := client.GetInfo()
			Expect(err).NotTo(HaveOccurred())

			Expect(info.Version).To(Equal("12.3.4"))
		})
	})
})
