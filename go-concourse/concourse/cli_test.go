package concourse_test

import (
	"io/ioutil"
	"net/http"
	"net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler CLI", func() {
	Describe("GetCLIReader", func() {
		var (
			expectedArch     string
			expectedPlatform string
		)

		BeforeEach(func() {
			expectedArch = "fake_arch"
			expectedPlatform = "fake_platform"

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						"/api/v1/cli",
						url.Values{
							"arch":     {expectedArch},
							"platform": {expectedPlatform},
						}.Encode(),
					),
					ghttp.RespondWith(http.StatusOK, "sup"),
				),
			)
		})

		It("returns an unclosed io.ReaderCloser", func() {
			readerCloser, _, err := client.GetCLIReader(expectedArch, expectedPlatform)
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(readerCloser)).To(Equal([]byte("sup")))
		})

		It("returns response Headers", func() {
			_, headers, err := client.GetCLIReader(expectedArch, expectedPlatform)
			Expect(err).NotTo(HaveOccurred())
			Expect(headers.Get("Content-Length")).To(Equal("3"))
		})
	})
})
