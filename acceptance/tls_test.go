package acceptance_test

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/lib/pq"
)

var _ = Describe("TLS", func() {
	var (
		atcCommand *ATCCommand
		dbListener *pq.Listener
	)

	BeforeEach(func() {
		postgresRunner.Truncate()
		dbConn = db.Wrap(postgresRunner.Open())

		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		bus := db.NewNotificationsBus(dbListener, dbConn)
		sqlDB = db.NewSQL(dbConn, bus)
	})

	AfterEach(func() {
		atcCommand.Stop()

		Expect(dbConn.Close()).To(Succeed())
		Expect(dbListener.Close()).To(Succeed())
	})

	It("accepts HTTPS requests", func() {
		atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{"--tls-bind-port", "--tls-cert", "--tls-key"}, DEVELOPMENT_MODE)
		err := atcCommand.Start()
		Expect(err).NotTo(HaveOccurred())

		request, err := http.NewRequest("GET", atcCommand.TLSURL(""), nil)
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
		Expect(resp.TLS.PeerCertificates[0].Issuer.Organization).To(ContainElement("Acme Co"))
	})

	It("redirects HTTP API traffic to HTTPS", func() {
		atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{"--tls-bind-port", "--tls-cert", "--tls-key"}, DEVELOPMENT_MODE)
		err := atcCommand.Start()
		Expect(err).NotTo(HaveOccurred())

		request, err := http.NewRequest("GET", atcCommand.URL("/api/v1/workers"), nil)

		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: transport}

		resp, err := client.Do(request)
		Expect(err).NotTo(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(resp.Request.URL.String()).To(Equal(atcCommand.TLSURL("/api/v1/workers")))
	})

	It("redirects HTTP web traffic to HTTPS", func() {
		atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{"--tls-bind-port", "--tls-cert", "--tls-key"}, DEVELOPMENT_MODE)
		err := atcCommand.Start()
		Expect(err).NotTo(HaveOccurred())

		request, err := http.NewRequest("GET", atcCommand.URL(""), nil)

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
		Expect(redirectURLs).To(ContainElement(atcCommand.TLSURL("/")))
	})

	It("redirects HTTP oauth traffic to HTTPS", func() {
		atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{"--tls-bind-port", "--tls-cert", "--tls-key"}, GITHUB_AUTH)
		err := atcCommand.Start()
		Expect(err).NotTo(HaveOccurred())

		request, err := http.NewRequest("GET", atcCommand.URL("/auth/github?team_name=main"), nil)

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
		Expect(redirectURLs[0]).To(Equal(atcCommand.TLSURL("/auth/github?team_name=main")))
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

		atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{"--tls-bind-port", "--tls-cert", "--tls-key"}, DEVELOPMENT_MODE)
		err = atcCommand.Start()
		Expect(err).NotTo(HaveOccurred())

		request, err := http.NewRequest("POST",
			atcCommand.URL("/api/v1/workers"),
			ioutil.NopCloser(bytes.NewBuffer(payload)),
		)
		Expect(err).NotTo(HaveOccurred())

		resp, err := http.DefaultClient.Do(request)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	It("validates certs on client side when not started in development mode", func() {
		atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{"--tls-bind-port", "--tls-cert", "--tls-key"}, BASIC_AUTH_NO_USERNAME, BASIC_AUTH_NO_PASSWORD)
		err := atcCommand.Start()
		Expect(err).NotTo(HaveOccurred())

		request, err := http.NewRequest("GET", atcCommand.URL(""), nil)

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
