package acceptance_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/db"
	"github.com/lib/pq"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("TLS", func() {
	var (
		atcProcess ifrit.Process
		dbListener *pq.Listener
		atcPort    uint16
		tlsPort    uint16
	)

	BeforeEach(func() {
		postgresRunner.Truncate()
		dbConn = db.Wrap(postgresRunner.Open())

		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		bus := db.NewNotificationsBus(dbListener, dbConn)
		sqlDB = db.NewSQL(dbConn, bus)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(atcProcess)

		Expect(dbConn.Close()).To(Succeed())
		Expect(dbListener.Close()).To(Succeed())
	})

	Describe("making an HTTPS request", func() {
		BeforeEach(func() {
			atcProcess, atcPort, tlsPort = startATC(atcBin, 1, true, []string{"--tls-bind-port", "--tls-cert", "--tls-key"}, DEVELOPMENT_MODE)
		})

		It("accepts the HTTPS connection", func() {
			request, err := http.NewRequest("GET", fmt.Sprintf("https://127.0.0.1:%d/", tlsPort), nil)

			transport := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // allow self-signed certificate
			}
			client := &http.Client{Transport: transport}

			resp, err := client.Do(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("making an HTTP request", func() {
		BeforeEach(func() {
			atcProcess, atcPort, tlsPort = startATC(atcBin, 1, true, []string{"--tls-bind-port", "--tls-cert", "--tls-key"}, DEVELOPMENT_MODE)
		})

		It("API handler redirects HTTP request to HTTPS external-url", func() {
			request, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/v1/workers", atcPort), nil)

			transport := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			client := &http.Client{Transport: transport}

			resp, err := client.Do(request)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Request.URL.String()).To(Equal("https://example.com"))
		})

		It("Web handler redirects HTTP request to HTTPS external-url", func() {
			request, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d", atcPort), nil)

			transport := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			client := &http.Client{Transport: transport}

			resp, err := client.Do(request)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Request.URL.String()).To(Equal("https://example.com"))
		})

		It("OAuth handler redirects HTTP request to HTTPS external-url", func() {
			request, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/v1/auth/token", atcPort), nil)

			transport := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			client := &http.Client{Transport: transport}

			resp, err := client.Do(request)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Request.URL.String()).To(Equal("https://example.com"))
		})
	})
})
