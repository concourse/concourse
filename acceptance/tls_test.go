package acceptance_test

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
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

	It("accepts HTTPS requests", func() {
		atcProcess, atcPort, tlsPort = startATC(atcBin, 1, []string{"--tls-bind-port", "--tls-cert", "--tls-key"}, DEVELOPMENT_MODE)
		request, err := http.NewRequest("GET", fmt.Sprintf("https://127.0.0.1:%d/", tlsPort), nil)
		Expect(err).NotTo(HaveOccurred())

		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}

		resp, err := client.Do(request)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(resp.TLS).NotTo(BeNil())
		Expect(resp.TLS.PeerCertificates).To(HaveLen(1))
		Expect(resp.TLS.PeerCertificates[0].Issuer.Organization).To(ContainElement(tlsCertificateOrganization))
	})

	It("redirects HTTP API traffic to HTTPS", func() {
		atcProcess, atcPort, tlsPort = startATC(atcBin, 1, []string{"--tls-bind-port", "--tls-cert", "--tls-key"}, DEVELOPMENT_MODE)

		request, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/v1/workers", atcPort), nil)

		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: transport}

		resp, err := client.Do(request)
		Expect(err).NotTo(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(resp.Request.URL.String()).To(Equal(fmt.Sprintf("https://127.0.0.1:%d/api/v1/workers", tlsPort)))
	})

	It("redirects HTTP web traffic to HTTPS", func() {
		atcProcess, atcPort, tlsPort = startATC(atcBin, 1, []string{"--tls-bind-port", "--tls-cert", "--tls-key"}, DEVELOPMENT_MODE)
		request, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d", atcPort), nil)

		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}

		redirectURLs := []string{}
		client := &http.Client{
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				redirectURLs = append(redirectURLs, req.URL.String())
				return nil
			},
		}

		resp, err := client.Do(request)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(len(redirectURLs)).To(Equal(1))
		Expect(redirectURLs).To(ContainElement(fmt.Sprintf("https://127.0.0.1:%d/", tlsPort)))
	})

	It("redirects HTTP oauth traffic to HTTPS", func() {
		atcProcess, atcPort, tlsPort = startATC(atcBin, 1, []string{"--tls-bind-port", "--tls-cert", "--tls-key"}, GITHUB_AUTH)

		request, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/auth/github?team_name=main", atcPort), nil)

		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}

		redirectURLs := []string{}
		client := &http.Client{
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				redirectURLs = append(redirectURLs, req.URL.String())
				return nil
			},
		}

		resp, err := client.Do(request)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(redirectURLs[0]).To(Equal(fmt.Sprintf("https://127.0.0.1:%d/auth/github?team_name=main", tlsPort)))
	})

	It("uses original handler for HTTP traffic that is not a GET or HEAD request when TLS is enabled", func() {
		worker := atc.Worker{
			Name:             "worker-name",
			GardenAddr:       "1.2.3.4:7777",
			BaggageclaimURL:  "5.6.7.8:7788",
			HTTPProxyURL:     "http://example.com",
			HTTPSProxyURL:    "https://example.com",
			NoProxy:          "example.com,127.0.0.1,localhost",
			ActiveContainers: 2,
			ResourceTypes: []atc.WorkerResourceType{
				{Type: "some-resource", Image: "some-resource-image"},
			},
			Platform: "haiku",
			Tags:     []string{"not", "a", "limerick"},
		}
		payload, err := json.Marshal(worker)
		Expect(err).NotTo(HaveOccurred())

		atcProcess, atcPort, tlsPort = startATC(atcBin, 1, []string{"--tls-bind-port", "--tls-cert", "--tls-key"}, DEVELOPMENT_MODE)

		request, err := http.NewRequest("POST",
			fmt.Sprintf("http://127.0.0.1:%d/api/v1/workers", atcPort),
			ioutil.NopCloser(bytes.NewBuffer(payload)),
		)
		Expect(err).NotTo(HaveOccurred())

		resp, err := http.DefaultClient.Do(request)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	It("validates certs on client side when not started in development mode", func() {
		atcProcess, atcPort, tlsPort = startATC(atcBin, 1, []string{"--tls-bind-port", "--tls-cert", "--tls-key"}, BASIC_AUTH_NO_USERNAME, BASIC_AUTH_NO_PASSWORD)
		request, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d", atcPort), nil)

		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}

		client := &http.Client{
			Transport: transport,
		}

		resp, err := client.Do(request)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
	})
})
