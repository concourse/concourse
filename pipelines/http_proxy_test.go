package pipelines_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A pipeline containing a job that hits a url behind a proxy", func() {
	It("uses the proxy server", func() {
		workers, err := client.ListWorkers()
		Expect(err).NotTo(HaveOccurred())

		var hasProxyWorker bool
	dance:
		for _, worker := range workers {
			for _, tag := range worker.Tags {
				if tag == "proxy" {
					hasProxyWorker = true
					break dance
				}
			}
		}

		if !hasProxyWorker {
			Skip("this only runs when a worker with the 'proxy' tag is available")
		}

		cmd := exec.Command(flyBin, []string{
			"-t", targetedConcourse,
			"execute",
			"-c", "fixtures/http-proxy-task.yml",
			"--tag", "proxy"}...,
		)

		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		<-session.Exited

		Expect(session).To(gbytes.Say("proxy.example.com"))
	})
})
