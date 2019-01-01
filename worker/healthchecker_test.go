package worker_test

import (
	"net/http"
	"net/http/httptest"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/onsi/gomega/ghttp"

	. "github.com/concourse/concourse/worker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckHealth", func() {
	var (
		healthchecker *httptest.Server
		garden        *ghttp.Server
		baggageclaim  *ghttp.Server

		testLogger = lagertest.NewTestLogger("healthchecker")
	)

	BeforeEach(func() {
		garden = ghttp.NewServer()
		baggageclaim = ghttp.NewServer()

		hc := NewHealthChecker(testLogger,
			"http://"+baggageclaim.Addr(), "http://"+garden.Addr(), 100*time.Millisecond)

		healthchecker = httptest.NewServer(
			http.HandlerFunc(hc.CheckHealth))
	})

	AfterEach(func() {
		baggageclaim.Close()
		garden.Close()
		healthchecker.Close()
	})

	Context("when receiving a request", func() {
		var (
			resp *http.Response
			err  error
		)

		JustBeforeEach(func() {
			resp, err = http.Get(healthchecker.URL)
			Expect(err).ToNot(HaveOccurred())
		})

		BeforeEach(func() {
			garden.AppendHandlers(
				ghttp.RespondWithJSONEncoded(200, map[string]string{}),
				ghttp.RespondWithJSONEncoded(200, map[string]string{}))

			baggageclaim.AppendHandlers(
				ghttp.RespondWithJSONEncoded(200, map[string]string{}),
				ghttp.RespondWithJSONEncoded(200, map[string]string{}))
		})

		It("makes requests to baggageclaim", func() {
			Expect(baggageclaim.ReceivedRequests()).To(HaveLen(2))
		})

		It("makes requests to garden", func() {
			Expect(garden.ReceivedRequests()).To(HaveLen(2))
		})

		Context("having a very slow baggaclaim", func() {
			BeforeEach(func() {
				baggageclaim.Reset()
				baggageclaim.AppendHandlers(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(1 * time.Second)
				})
			})

			It("doesn't wait forever", func() {
				Expect(resp.StatusCode).To(Equal(503))
			})
		})

		Context("having baggageclaim down", func() {
			BeforeEach(func() {
				baggageclaim.Close()
			})

			It("returns 503", func() {
				Expect(resp.StatusCode).To(Equal(503))
			})
		})

		Context("having garden down", func() {
			BeforeEach(func() {
				garden.Close()
			})

			It("returns 503", func() {
				Expect(resp.StatusCode).To(Equal(503))
			})
		})

		Context("having baggageclaim AND garden up", func() {
			It("returns 200", func() {
				Expect(resp.StatusCode).To(Equal(200))
			})
		})
	})
})
