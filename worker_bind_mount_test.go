package topgun_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = XDescribe("Bind mount certificates", func() {
	Context("when atc cert is appended to certificates in the worker", func() {
		var inputDir string
		var outputDir string

		BeforeEach(func() {
			Deploy("deployments/ca-cert-atc.yml")

			bosh("scp", "certs/atc-cert.crt", "worker:/tmp")
			bosh("ssh", "worker", "-c", `sudo sh -c "mv /tmp/atc-cert.crt /usr/local/share/ca-certificates/"`)
			bosh("ssh", "worker", "-c", `sudo update-ca-certificates`)

			<-spawnFly("login", "-c", atcExternalURL, "--ca-cert", "certs/atc-cert.crt").Exited

			var err error
			inputDir, err = ioutil.TempDir("", "topgun")
			Expect(err).NotTo(HaveOccurred())

			outputDir, err = ioutil.TempDir("", "topgun")
			Expect(err).NotTo(HaveOccurred())

			// restart to re-read certs in beacon_ctl
			bosh("restart", "worker")
		})

		AfterEach(func() {
			err := os.RemoveAll(inputDir)
			Expect(err).NotTo(HaveOccurred())
			err = os.RemoveAll(outputDir)
			Expect(err).NotTo(HaveOccurred())
		})

		It("curls the atc ip using the cert mounted to the container from the worker", func() {
			By("setting pipeline with get, check and put")
			fly("set-pipeline", "-n", "-c", "pipelines/curl-check.yml", "-p", "bind-mount-certs")

			By("unpausing the pipeline")
			fly("unpause-pipeline", "-p", "bind-mount-certs")

			By("triggering check")
			<-spawnFly("check-resource", "-r", "bind-mount-certs/simple-resource").Exited

			By("hijacking into check container")
			hijackSession := spawnFly("hijack", "-c", "bind-mount-certs/simple-resource", "--", "/bin/sh", "-c", "curl "+atcExternalURLTLS)
			<-hijackSession.Exited
			Expect(hijackSession.ExitCode()).To(Equal(0))

			By("triggering a build")
			buildSession := spawnFly("execute", "-c", "tasks/input-output.yml", "-i", fmt.Sprintf("some-input=%s", inputDir), "-o", fmt.Sprintf("some-output=%s", outputDir))
			Eventually(buildSession).Should(gbytes.Say("executing build"))
			buildRegex := regexp.MustCompile(`executing build (\d+)`)
			matches := buildRegex.FindSubmatch(buildSession.Out.Contents())
			buildID := string(matches[1])
			<-buildSession.Exited
			Expect(buildSession.ExitCode()).To(Equal(0))

			By("hijacking into get container")
			hijackSession = spawnFly("hijack", "-b", buildID, "-s", "some-input", "--", "/bin/sh", "-c", "curl "+atcExternalURLTLS)
			<-hijackSession.Exited
			Expect(hijackSession.ExitCode()).To(Equal(0))

			By("hijacking into put container")
			hijackSession = spawnFly("hijack", "-b", buildID, "-s", "some-output", "--", "/bin/sh", "-c", "curl "+atcExternalURLTLS)
			<-hijackSession.Exited
			Expect(hijackSession.ExitCode()).To(Equal(0))
		})
	})
})
