package k8s_test

import (
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/square/certstrap/pkix"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Web HTTP or HTTPS(TLS) termination at web node", func() {
	var (
		serverCertBytes []byte
		serverKeyBytes  []byte
		caCertFile      *os.File
	)

	BeforeEach(func() {
		var err error

		CACert, serverKey, serverCert := generateKeyPairWithCA()
		CACertBytes, err := CACert.Export()
		Expect(err).NotTo(HaveOccurred())

		caCertFile, err = ioutil.TempFile("", "ca")
		caCertFile.Write(CACertBytes)
		caCertFile.Close()

		serverKeyBytes, err = serverKey.ExportPrivate()
		Expect(err).NotTo(HaveOccurred())

		serverCertBytes, err = serverCert.Export()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.Remove(caCertFile.Name())
	})

	Context("When configured correctly", func() {
		var (
			proxySession  *gexec.Session
			atcEndpoint   string
			chartConfig   []string
			proxyPort     string
			proxyProtocol string
		)

		JustBeforeEach(func() {
			setReleaseNameAndNamespace("wtt")
			deployConcourseChart(releaseName,
				chartConfig...)

			waitAllPodsInNamespaceToBeReady(namespace)

			By("Creating the web proxy")
			proxySession, atcEndpoint = startPortForwardingWithProtocol(namespace, "service/"+releaseName+"-web", proxyPort, proxyProtocol)
		})

		AfterEach(func() {
			cleanup(releaseName, namespace, proxySession)
		})

		Context("configure helm chart for tls termination at web", func() {

			BeforeEach(func() {
				chartConfig = generateChartConfig(
					"--set=concourse.web.externalUrl=https://test.com",
					"--set=concourse.web.tls.enabled=true",
					"--set=secrets.webTlsCert="+string(serverCertBytes),
					"--set=secrets.webTlsKey="+string(serverKeyBytes),
				)
				proxyPort = "443"
				proxyProtocol = "https"
			})

			It("fly login succeeds when using the correct CA and host", func() {
				By("Logging in")
				sess := fly.Start("login", "-u", "test", "-p", "test", "--ca-cert", caCertFile.Name(), "-c", atcEndpoint)
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})

			It("fly login fails when NOT using the correct CA", func() {
				By("Logging in")
				sess := fly.Start("login", "-u", "test", "-p", "test", "--ca-cert", "certs/wrong-ca.crt", "-c", atcEndpoint)
				<-sess.Exited
				Expect(sess.ExitCode()).ToNot(Equal(0))
				Expect(sess.Err).To(gbytes.Say(`x509: certificate signed by unknown authority`))
			})
		})

		Context("DON'T configure tls termination at web in helm chart", func() {
			BeforeEach(func() {
				chartConfig = generateChartConfig("--set=concourse.web.externalUrl=http://test.com",
					"--set=concourse.web.tls.enabled=false")
				proxyPort = "8080"
				proxyProtocol = "http"
			})
			It("fly login succeeds when connecting to web over http", func() {
				By("Logging in")
				fly.Login("test", "test", atcEndpoint)
			})
		})
	})

	Context("When NOT configured correctly", func() {

		BeforeEach(func() {
			setReleaseNameAndNamespace("wtt")
		})

		It("helm deploy fails if tls is enabled but externalURL is NOT set", func() {
			expectedErr := "Must specify HTTPS external URL when concourse.web.tls.enabled is true"
			chartConfig := generateChartConfig(
				"--set=concourse.web.tls.enabled=true",
				"--set=secrets.webTlsCert="+string(serverCertBytes),
				"--set=secrets.webTlsKey="+string(serverKeyBytes),
			)
			deployFailingConcourseChart(releaseName, expectedErr,
				chartConfig...,
			)
		})

		It("helm deploy fails when tls is enabled but ssl cert and ssl key are NOT set", func() {
			expectedErr := "secrets.webTlsCert is required because secrets.create is true and concourse.web.tls.enabled is true"
			chartConfig := generateChartConfig("--set=concourse.web.externalUrl=https://test.com",
				"--set=concourse.web.tls.enabled=true")
			deployFailingConcourseChart(releaseName, expectedErr,
				chartConfig...,
			)
		})
	})

})

func generateChartConfig(args ...string) []string {
	return append(args,
		"--set=worker.replicas=1",
		"--set=concourse.worker.baggageclaim.driver=detect",
		"--set=concourse.web.tls.bindPort=443",
	)
}
func generateKeyPairWithCA() (*pkix.Certificate, *pkix.Key, *pkix.Certificate) {
	CAKey, err := pkix.CreateRSAKey(1024)
	Expect(err).NotTo(HaveOccurred())

	CACert, err := pkix.CreateCertificateAuthority(CAKey, "", time.Now().Add(time.Hour), "Pivotal", "", "", "", "CA")
	Expect(err).NotTo(HaveOccurred())

	serverKey, err := pkix.CreateRSAKey(1024)
	Expect(err).NotTo(HaveOccurred())

	certificateSigningRequest, err := pkix.CreateCertificateSigningRequest(serverKey, "", []net.IP{net.IPv4(127, 0, 0, 1)},
		nil, "Pivotal", "", "", "", "127.0.0.1")
	Expect(err).NotTo(HaveOccurred())

	serverCert, err := pkix.CreateCertificateHost(CACert, CAKey, certificateSigningRequest, time.Now().Add(time.Hour))
	Expect(err).NotTo(HaveOccurred())

	return CACert, serverKey, serverCert
}
