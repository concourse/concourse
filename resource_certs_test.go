package topgun_test

import (
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource Certs", func() {
	BeforeEach(func() {
		Deploy("deployments/concourse-different-workers.yml",
			"-o", "operations/other-worker-no-certs.yml",
		)
		waitForWorkersToBeRunning()
	})

	Context("with a certs path configured on the resource", func() {
		var boshCerts string

		BeforeEach(func() {
			By("setting a pipeline that has a tagged resource")
			fly("set-pipeline", "-n", "-c", "pipelines/certs-tagged-resources.yml", "-p", "resources")

			By("unpausing the pipeline pipeline")
			fly("unpause-pipeline", "-p", "resources")
			certSession := bosh("ssh", "worker", "-c", "ls /etc/ssl/certs")
			<-certSession.Exited
			boshCerts = string(certSession.Out.Contents())
		})

		It("bind mounts the certs volume if the worker has one", func() {
			By("running the checks")
			fly("check-resource", "-r", "resources/no-certs")
			fly("check-resource", "-r", "resources/certs")

			hijackSession := spawnFly("hijack", "-c", "resources/no-certs", "--", "ls", "/etc/ssl/certs")
			<-hijackSession.Exited

			certsContent := string(hijackSession.Out.Contents())
			Expect(certsContent).To(HaveLen(0))

			hijackSession = spawnFly("hijack", "-c", "resources/certs", "--", "ls", "/etc/ssl/certs")
			<-hijackSession.Exited

			certsContent = string(hijackSession.Out.Contents())
			Expect(certsContent).ToNot(HaveLen(0))
		})
	})
})
