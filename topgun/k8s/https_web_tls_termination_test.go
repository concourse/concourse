package k8s_test

import (
	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"io/ioutil"
)

var (
	Cert string
	Key  string
)

var _ = Describe("Web HTTP or TLS termination at web node", func() {
	var (
		proxySession *gexec.Session
		atcEndpoint  string
		chartConfig  []string
		proxyPort    string
		proxyProtocol     string
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
		helmDestroy(releaseName)
		Wait(Start(nil, "kubectl", "delete", "namespace", namespace, "--wait=false"))
		Wait(proxySession.Interrupt())
	})

	Context("configure helm chart for tls termination at web", func() {
		BeforeEach(func() {
			buffCert, err := ioutil.ReadFile("certs/ssl.crt")
			Expect(err).ToNot(HaveOccurred())
			Cert = string(buffCert)
			buffKey, err := ioutil.ReadFile("certs/ssl.key")
			Key = string(buffKey)
			Expect(err).ToNot(HaveOccurred())
			chartConfig = generateChartConfig(
				"--set=concourse.web.externalUrl=https://test.com",
				"--set=concourse.web.tls.enabled=true",
				"--set=secrets.webTlsCert=" + Cert,
				"--set=secrets.webTlsKey=" + Key,
			)
			proxyPort = "443"
			proxyProtocol = "https"
		})
		It("fly login succeeds when using the correct CA and host", func() {
			By("Logging in")
			sess := fly.Start("login", "-u", "test", "-p", "test", "--ca-cert", "certs/ca.crt", "-c", atcEndpoint)
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
			fly.Login("test","test", atcEndpoint )
		})
	})
})

var _ = Describe("When TLS is not configured correctly", func() {
	BeforeEach(func() {
		setReleaseNameAndNamespace("wtt")
	})

	It("helm deploy fails if tls is enabled but externalURL is NOT set", func() {
		expectedErr := "Must specify HTTPS external URL when concourse.web.tls.enabled is true"
		chartConfig := generateChartConfig("--set=concourse.web.tls.enabled=true")
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

func generateChartConfig(args ...string) []string {
	return append(args,
		"--set=worker.replicas=1",
		"--set=concourse.worker.baggageclaim.driver=detect",
		"--set=concourse.web.tls.bindPort=443",
	)
}

