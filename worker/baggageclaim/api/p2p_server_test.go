package api_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"runtime"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/worker/baggageclaim/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("P2P Server", func() {
	var (
		handler http.Handler
		infc    string
	)

	JustBeforeEach(func() {
		var err error
		logger := lagertest.NewTestLogger("p2p-server")
		re := regexp.MustCompile(infc)
		handler, err = api.NewHandler(logger, nil, nil, re, 4, 7766)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("get p2p url", func() {
		var (
			request  *http.Request
			recorder *httptest.ResponseRecorder
		)
		JustBeforeEach(func() {
			var err error
			request, err = http.NewRequest("GET", "/p2p-url", nil)
			Expect(err).NotTo(HaveOccurred())

			recorder = httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
		})

		Context("when a valid interface name is given", func() {
			BeforeEach(func() {
				infc = "lo"
				if runtime.GOOS == "windows" {
					infc = "Loopback"
				}
			})

			It("returns a url successfully", func() {
				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(Equal("http://127.0.0.1:7766"))
			})
		})

		Context("when an invalid interface name is given", func() {
			BeforeEach(func() {
				infc = "dummy_interface"
			})

			It("returns a url successfully", func() {
				Expect(recorder.Code).To(Equal(500))
			})
		})
	})
})
