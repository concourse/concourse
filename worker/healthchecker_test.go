package worker_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/onsi/gomega/ghttp"

	. "github.com/concourse/concourse/worker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type componentHealth struct {
	Healthy       bool   `json:"healthy"`
	ResponseError string `json:"response_error"`
}

type healthResponseBody struct {
	Garden       componentHealth `json:"garden"`
	Baggageclaim componentHealth `json:"baggageclaim"`
}

func parseHealthResponse(resp *http.Response) healthResponseBody {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())
	var result healthResponseBody
	Expect(json.Unmarshal(body, &result)).To(Succeed())
	return result
}

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
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/ping"),
					ghttp.RespondWithJSONEncoded(200, map[string]string{}),
				),
			)

			baggageclaim.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/volumes"),
					ghttp.RespondWithJSONEncoded(200, []string{}),
				),
			)
		})

		It("makes an underlying request to baggageclaim", func() {
			Expect(baggageclaim.ReceivedRequests()).To(HaveLen(1))
		})

		It("makes an underlying request to garden", func() {
			Expect(garden.ReceivedRequests()).To(HaveLen(1))
		})

		Context("having a very slow baggaclaim", func() {
			BeforeEach(func() {
				baggageclaim.Reset()
				baggageclaim.AppendHandlers(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(1 * time.Second)
				})
			})

			It("doesn't wait forever", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusServiceUnavailable))
				body := parseHealthResponse(resp)
				Expect(body.Garden.Healthy).To(BeTrue())
				Expect(body.Baggageclaim.Healthy).To(BeFalse())
				Expect(body.Baggageclaim.ResponseError).ToNot(BeEmpty())
			})
		})

		Context("having baggageclaim down", func() {
			BeforeEach(func() {
				baggageclaim.Close()
			})

			It("returns 503 with baggageclaim unhealthy", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusServiceUnavailable))
				body := parseHealthResponse(resp)
				Expect(body.Garden.Healthy).To(BeTrue())
				Expect(body.Garden.ResponseError).To(BeEmpty())
				Expect(body.Baggageclaim.Healthy).To(BeFalse())
				Expect(body.Baggageclaim.ResponseError).ToNot(BeEmpty())
			})
		})

		Context("having garden down", func() {
			BeforeEach(func() {
				garden.Close()
			})

			It("returns 503 with garden unhealthy", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusServiceUnavailable))
				body := parseHealthResponse(resp)
				Expect(body.Garden.Healthy).To(BeFalse())
				Expect(body.Garden.ResponseError).ToNot(BeEmpty())
			})
		})

		Context("having baggageclaim AND garden up", func() {
			It("returns 200 with json response", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/json"))
				body := parseHealthResponse(resp)
				Expect(body.Garden.Healthy).To(BeTrue())
				Expect(body.Garden.ResponseError).To(BeEmpty())
				Expect(body.Baggageclaim.Healthy).To(BeTrue())
				Expect(body.Baggageclaim.ResponseError).To(BeEmpty())
			})
		})
	})
})
