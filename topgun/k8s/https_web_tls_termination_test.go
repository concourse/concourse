package k8s_test

import (
	"net"
	"os"
	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/square/certstrap/pkix"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Web HTTP or HTTPS(TLS) termination at web node", func() {

	var (
		serverCertBytes         []byte
		serverKeyBytes          []byte
		caCertFile              *os.File
		incorrectCaCertFilePath string
	)

	BeforeEach(func() {
		setReleaseNameAndNamespace("wtt")

		By("creating a self-signed cert for the web node", func() {
			CACert, serverKey, serverCert := generateKeyPairWithCA(namespace, releaseName+"-web")
			CACertBytes, err := CACert.Export()
			Expect(err).NotTo(HaveOccurred())

			caCertFile, err = os.CreateTemp("", "ca")
			Expect(err).NotTo(HaveOccurred())
			caCertFile.Write(CACertBytes)
			caCertFile.Close()

			serverKeyBytes, err = serverKey.ExportPrivate()
			Expect(err).NotTo(HaveOccurred())

			serverCertBytes, err = serverCert.Export()
			Expect(err).NotTo(HaveOccurred())
		})

		By("creating a self-signed cert not used by the web node", func() {
			CACert, _, _ := generateKeyPairWithCA(namespace, releaseName+"-web")
			CACertBytes, err := CACert.Export()
			Expect(err).NotTo(HaveOccurred())

			unknownCaCertFile, err := os.CreateTemp("", "ca")
			Expect(err).NotTo(HaveOccurred())
			unknownCaCertFile.Write(CACertBytes)
			unknownCaCertFile.Close()
			incorrectCaCertFilePath = unknownCaCertFile.Name()
		})
	})

	AfterEach(func() {
		os.Remove(caCertFile.Name())
	})

	Context("when configured correctly", func() {

		var (
			atc Endpoint

			chartConfig []string
			proxyPort   string
		)

		JustBeforeEach(func() {
			deployConcourseChart(releaseName, chartConfig...)

			waitAllPodsInNamespaceToBeReady(namespace)

			atc = endpointFactory.NewServiceEndpoint(
				namespace,
				releaseName+"-web",
				proxyPort,
			)
		})

		AfterEach(func() {
			atc.Close()
			cleanupReleases()
		})

		Context("with tls termination at web", func() {

			BeforeEach(func() {
				chartConfig = generateChartConfig(
					"--set=concourse.web.externalUrl=https://test.com",
					"--set=concourse.web.tls.enabled=true",
					"--set=secrets.webTlsCert="+string(serverCertBytes),
					"--set=secrets.webTlsKey="+string(serverKeyBytes),
				)

				proxyPort = "443"
			})

			It("fly login succeeds when using the correct CA and host", func() {
				fly.Login(
					"test",
					"test",
					"https://"+atc.Address(),
					"--ca-cert", caCertFile.Name(),
				)
			})

			It("fly login fails when NOT using the correct CA", func() {
				Eventually(func() *gbytes.Buffer {
					sess := fly.Start("login", "-u", "test", "-p", "test",
						"--ca-cert", incorrectCaCertFilePath,
						"-c", "https://"+atc.Address(),
					)
					Eventually(sess).Should(gexec.Exit())
					return sess.Err
				}, 2*time.Minute, 10*time.Second).
					Should(gbytes.Say(`x509: certificate signed by unknown authority`))
			})
		})

	})

	Context("When NOT configured correctly", func() {

		BeforeEach(func() {
			setReleaseNameAndNamespace("wtt")
		})

		It("fails if tls is enabled but externalURL is NOT set", func() {
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

		It("fails when tls is enabled but ssl cert and ssl key are NOT set", func() {
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
		"--set=worker.enabled=false",
		"--set=concourse.worker.baggageclaim.driver=detect",
		"--set=concourse.web.tls.bindPort=443",
	)
}
func generateKeyPairWithCA(namespace, service string) (*pkix.Certificate, *pkix.Key, *pkix.Certificate) {
	CAKey, err := pkix.CreateRSAKey(1024)
	Expect(err).NotTo(HaveOccurred())

	CACert, err := pkix.CreateCertificateAuthority(CAKey, "", time.Now().Add(time.Hour), "Pivotal", "", "", "", "CA", []string{})
	Expect(err).NotTo(HaveOccurred())

	serverKey, err := pkix.CreateRSAKey(1024)
	Expect(err).NotTo(HaveOccurred())

	certificateSigningRequest, err := pkix.CreateCertificateSigningRequest(
		serverKey, "", []net.IP{net.IPv4(127, 0, 0, 1)},
		[]string{serviceAddress(namespace, service)}, nil,
		"Pivotal", "", "", "", "127.0.0.1")
	Expect(err).NotTo(HaveOccurred())

	serverCert, err := pkix.CreateCertificateHost(CACert, CAKey,
		certificateSigningRequest, time.Now().Add(time.Hour))
	Expect(err).NotTo(HaveOccurred())

	return CACert, serverKey, serverCert
}
