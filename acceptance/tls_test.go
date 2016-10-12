package acceptance_test

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
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

		pgxConn := postgresRunner.OpenPgx()
		fakeConnector := new(dbfakes.FakeConnector)
		retryableConn := &db.RetryableConn{Connector: fakeConnector, Conn: pgxConn}

		lockFactory := db.NewLockFactory(retryableConn)
		sqlDB = db.NewSQL(dbConn, bus, lockFactory)
	})

	AfterEach(func() {
		atcCommand.Stop()

		Expect(dbConn.Close()).To(Succeed())
		Expect(dbListener.Close()).To(Succeed())
	})

	authorizedTLSClient := func(atcCommand *ATCCommand) *http.Client {
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}

		request, err := http.NewRequest("GET", atcCommand.TLSURL("/api/v1/teams/main/auth/token"), nil)
		resp, err := client.Do(request)
		Expect(err).NotTo(HaveOccurred())

		defer resp.Body.Close()
		var atcToken atc.AuthToken
		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())

		err = json.Unmarshal(body, &atcToken)
		Expect(err).NotTo(HaveOccurred())

		return &http.Client{
			Transport: &oauth2.Transport{
				Source: oauth2.StaticTokenSource(&oauth2.Token{
					TokenType:   atcToken.Type,
					AccessToken: atcToken.Value,
				}),
				Base: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
			},
		}
	}

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

	It("does not redirect HTTP API traffic to HTTPS", func() {
		atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{"--tls-bind-port", "--tls-cert", "--tls-key"}, DEVELOPMENT_MODE)
		err := atcCommand.Start()
		Expect(err).NotTo(HaveOccurred())

		request, err := http.NewRequest("GET", atcCommand.URL("/api/v1/info"), nil)
		Expect(err).NotTo(HaveOccurred())

		client := authorizedTLSClient(atcCommand)
		resp, err := client.Do(request)
		Expect(err).NotTo(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(resp.Request.URL.String()).To(Equal(atcCommand.URL("/api/v1/info")))
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
})
